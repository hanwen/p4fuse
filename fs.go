// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

/* TODO

- readlink.
- expose md5 as xattr.

*/

import (
	"crypto"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	_ "crypto/md5"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/p4fuse/p4"
)

type P4Fs struct {
	backingDir string
	root       *p4Root
	p4         *p4.Conn
}

// Creates a new P4FS
func NewP4FSRoot(conn *p4.Conn, backingDir string) nodefs.Node {
	fs := &P4Fs{
		p4:         conn,
	}

	fs.backingDir = backingDir
	fs.root = &p4Root{
		Node: nodefs.NewDefaultNode(),
		fs:   fs,
	}
	return fs.root
}

func (fs *P4Fs) onMount() {
	fs.root.Inode().NewChild("head", false, fs.newP4Link())
}

func (fs *P4Fs) newFolder(path string, change int) *p4Folder {
	return &p4Folder{
		Node:   nodefs.NewDefaultNode(),
		fs:     fs,
		path:   path,
		change: change,
	}
}

func (fs *P4Fs) newFile(st *p4.Stat) *p4File {
	f := &p4File{Node: nodefs.NewDefaultNode(), fs: fs, stat: *st}
	return f
}

func (fs *P4Fs) newP4Link() *p4Link {
	return &p4Link{
		Node: nodefs.NewDefaultNode(),
		fs:   fs,
	}
}

////////////////
type p4Link struct {
	nodefs.Node
	fs *P4Fs
}

func (f *p4Link) Deletable() bool {
	return false
}

func (f *p4Link) GetAttr(out *fuse.Attr, file nodefs.File, c *fuse.Context) fuse.Status {
	out.Mode = fuse.S_IFLNK
	return fuse.OK
}

func (f *p4Link) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	r, err := f.fs.p4.Changes([]string{"-s", "submitted", "-m1"})
	if err != nil {
		log.Printf("p4.Changes: %v", err)
		return nil, fuse.EIO
	}

	ch := r[0].(*p4.Change)
	return []byte(fmt.Sprintf("%d", ch.Change)), fuse.OK
}

type p4Root struct {
	nodefs.Node
	fs *P4Fs

	link *p4Link
}

func (r *p4Root) OnMount(conn *nodefs.FileSystemConnector) {
}

func (f *p4Root) OpenDir(context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	return []fuse.DirEntry{{Name: "head", Mode: fuse.S_IFLNK}}, fuse.OK
}

func (r *p4Root) Lookup(out *fuse.Attr, name string, context *fuse.Context) (node *nodefs.Inode, code fuse.Status) {
	cl, err := strconv.ParseInt(name, 10, 64)
	if err != nil {
		return nil, fuse.ENOENT
	}

	fsNode := r.fs.newFolder("", int(cl))
	fsNode.GetAttr(out, nil, context)
	return r.Inode().NewChild(name, true, fsNode), fuse.OK
}

////////////////

type p4Folder struct {
	nodefs.Node
	change int
	path   string
	fs     *P4Fs

	// nil means they haven't been fetched yet.
	mu      sync.Mutex
	files   map[string]*p4.Stat
	folders map[string]bool
}

func (f *p4Folder) OpenDir(context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	if !f.fetch() {
		return nil, fuse.EIO
	}
	stream = make([]fuse.DirEntry, 0, len(f.files)+len(f.folders))

	for n, _ := range f.files {
		mode := fuse.S_IFREG | 0644
		stream = append(stream, fuse.DirEntry{Name: n, Mode: uint32(mode)})
	}
	for n, _ := range f.folders {
		mode := fuse.S_IFDIR | 0755
		stream = append(stream, fuse.DirEntry{Name: n, Mode: uint32(mode)})
	}
	return stream, fuse.OK
}

func (f *p4Folder) GetAttr(out *fuse.Attr, file nodefs.File, c *fuse.Context) fuse.Status {
	out.Mode = fuse.S_IFDIR | 0755
	return fuse.OK
}

func (f *p4Folder) Deletable() bool {
	return false
}

func (f *p4Folder) fetch() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files != nil {
		return true
	}

	var err error
	path := "//" + f.path
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	path += fmt.Sprintf("*@%d", f.change)

	folders, err := f.fs.p4.Dirs([]string{path})
	if err != nil {
		log.Printf("fetch: %v", err)
		return false
	}
	files, err := f.fs.p4.Fstat([]string{path})
	if err != nil {
		log.Printf("fetch: %v", err)
		return false
	}

	f.files = map[string]*p4.Stat{}
	for _, r := range files {
		if stat, ok := r.(*p4.Stat); ok && stat.HeadAction != "delete" {
			_, base := filepath.Split(stat.DepotFile)
			f.files[base] = stat
		}
	}

	f.folders = map[string]bool{}
	for _, r := range folders {
		if dir, ok := r.(*p4.Dir); ok {
			_, base := filepath.Split(dir.Dir)
			f.folders[base] = true
		}
	}

	return true
}

func (f *p4Folder) Lookup(out *fuse.Attr, name string, context *fuse.Context) (*nodefs.Inode, fuse.Status) {
	f.fetch()

	var node nodefs.Node
	if st := f.files[name]; st != nil {
		node = f.fs.newFile(st)
	} else if f.folders[name] {
		node = f.fs.newFolder(filepath.Join(f.path, name), f.change)
	} else {
		return nil, fuse.ENOENT
	}

	node.GetAttr(out, nil, context)
	return f.Inode().NewChild(name, true, node), fuse.OK
}

////////////////

type p4File struct {
	nodefs.Node
	stat p4.Stat
	fs   *P4Fs

	mu      sync.Mutex
	backing string
}

var modes = map[string]uint32{
	"xtext":   fuse.S_IFREG | 0755,
	"xbinary": fuse.S_IFREG | 0755,
	"kxtext":  fuse.S_IFREG | 0755,
	"symlink": fuse.S_IFLNK | 0777,
}

func (f *p4File) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	id := fmt.Sprintf("%s#%d", f.stat.DepotFile, f.stat.HeadRev)
	content, err := f.fs.p4.Print(id)
	if err != nil {
		log.Printf("p4 print: %v", err)
		return nil, fuse.EIO
	}
	if len(content) == 0 || content[len(content)-1] != '\n' {
		log.Printf("terminating newline for symlink missing: %q", content)
		return nil, fuse.EIO
	}
	return content[:len(content)-1], fuse.OK
}

func (f *p4File) GetAttr(out *fuse.Attr, file nodefs.File, c *fuse.Context) fuse.Status {
	if m, ok := modes[f.stat.HeadType]; ok {
		out.Mode = m
	} else {
		out.Mode = fuse.S_IFREG | 0644
	}

	out.Mtime = uint64(f.stat.HeadTime)
	out.Size = uint64(f.stat.FileSize)
	return fuse.OK
}

func (f *p4File) fetch() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.backing != "" {
		return true
	}
	id := fmt.Sprintf("%s#%d", f.stat.DepotFile, f.stat.HeadRev)
	h := crypto.MD5.New()
	h.Write([]byte(id))
	sum := fmt.Sprintf("%x", h.Sum(nil))
	dir := filepath.Join(f.fs.backingDir, sum[:2])
	_, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		os.Mkdir(dir, 0700)
	}

	dest := fmt.Sprintf("%s/%x", dir, sum[2:])
	if _, err := os.Lstat(dest); err == nil {
		f.backing = dest
		return true
	}
	content, err := f.fs.p4.Print(id)
	if err != nil {
		log.Printf("p4 print error: %v", err)
		return false
	}

	tmp, err := ioutil.TempFile(f.fs.backingDir, "")
	if err != nil {
		log.Printf("TempFile: %v", err)
		return false
	}

	tmp.Write(content)
	tmp.Close()

	os.Rename(tmp.Name(), dest)
	f.backing = dest
	return true
}

func (f *p4File) Deletable() bool {
	return false
}

func (n *p4File) Open(flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EROFS
	}

	n.fetch()
	f, err := os.OpenFile(n.backing, int(flags), 0644)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	return nodefs.NewLoopbackFile(f), fuse.OK
}
