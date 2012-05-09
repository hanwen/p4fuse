// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

/* TODO

 - symlinks.
 - expose md5 as xattr.
 - head symlink.

*/
	
import (
	"crypto"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"strconv"
	
	"github.com/hanwen/go-fuse/fuse"
	
	_ "crypto/md5"
)

type P4Fs struct {
	fuse.DefaultNodeFileSystem
	backingDir string
	root *p4Root
	p4 *P4
}

// Creates a new P4FS
func NewP4Fs(p4 *P4, backingDir string) *P4Fs {
	fs := &P4Fs{
		p4: p4,
	}

	fs.backingDir = backingDir
	fs.root = &p4Root{fs: fs}
	return fs
}


func (fs *P4Fs) Root() fuse.FsNode {
	return fs.root
}

func (fs *P4Fs) newFolder(path string, change int) *p4Folder {
	f := &p4Folder{fs: fs, path: path, change: change}
	return f
}

func (fs *P4Fs) newFile(st *Stat) *p4File {
	f := &p4File{fs: fs, stat: *st}
	return f
}
////////////////

type p4Root struct {
	fuse.DefaultFsNode
	fs *P4Fs
}

func (r *p4Root) Lookup(name string, context *fuse.Context) (fi *fuse.Attr, node fuse.FsNode, code fuse.Status) {
	cl, err := strconv.ParseInt(name, 10, 64)
	if err != nil {
		return nil, nil, fuse.ENOENT
	}

	node = r.fs.newFolder("", int(cl))
	r.Inode().AddChild(name, r.Inode().New(true, node))

	a, _ := node.GetAttr(nil, context)
	return a, node, fuse.OK
}

func (f *p4Root) OpenDir(context *fuse.Context) (stream chan fuse.DirEntry, status fuse.Status) {
	stream = make(chan fuse.DirEntry, 1)
	close(stream)
	return stream, fuse.OK
}

////////////////


type p4Folder struct {
	fuse.DefaultFsNode
	change  int
	path string
	fs   *P4Fs

	// nil means they haven't been fetched yet.
	files    map[string]*Stat
	folders  map[string]bool
}

func (f *p4Folder) OpenDir(context *fuse.Context) (stream chan fuse.DirEntry, status fuse.Status) {
	if !f.fetch() {
		return nil, fuse.EIO
	}
	stream = make(chan fuse.DirEntry, len(f.files)+len(f.folders))
	
	for n, _ := range f.files {
		mode := fuse.S_IFREG | 0644
		stream <- fuse.DirEntry{Name: n, Mode: uint32(mode)}		
	}
	for n, _ := range f.folders {
		mode := fuse.S_IFDIR | 0755
		stream <- fuse.DirEntry{Name: n, Mode: uint32(mode)}		
	}
	close(stream)
	return stream, fuse.OK
}
		
func (f *p4Folder) GetAttr(file fuse.File, c *fuse.Context) (*fuse.Attr, fuse.Status) {
	return &fuse.Attr{
		Mode: fuse.S_IFDIR | 0755,
	}, fuse.OK
}

func (f *p4Folder) Deletable() bool {
	return false
}

func (f *p4Folder) fetch() bool {
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
	files, err  := f.fs.p4.Fstat([]string{path})
	if err != nil {
		log.Printf("fetch: %v", err)
		return false
	}

	f.files = map[string]*Stat{}
	for _, r := range files {
		if stat, ok := r.(*Stat); ok && stat.HeadAction != "delete" {
			_, base := filepath.Split(stat.DepotFile)
			f.files[base] = stat
		}
	}
	
	f.folders = map[string]bool{}
	for _, r := range folders {
		if dir, ok := r.(*Dir); ok {
			_, base := filepath.Split(dir.Dir)
			f.folders[base] = true
		}
	}
	
	return true
}

func (f *p4Folder) Lookup(name string, context *fuse.Context) (fi *fuse.Attr, node fuse.FsNode, code fuse.Status) {
	f.fetch()
	
	if st := f.files[name]; st != nil {
		node = f.fs.newFile(st)
	} else if f.folders[name] {
		node = f.fs.newFolder(filepath.Join(f.path, name), f.change)
	} else {
		return nil, nil, fuse.ENOENT
	}
	
	f.Inode().AddChild(name, f.Inode().New(true, node))

	a, _ := node.GetAttr(nil, context)
	return a, node, fuse.OK
}

	
////////////////

type p4File struct {
	fuse.DefaultFsNode
	stat Stat
	fs *P4Fs
	backing string
}

func (f *p4File) GetAttr(file fuse.File, c *fuse.Context) (*fuse.Attr, fuse.Status) {
	return &fuse.Attr{
		Size: uint64(f.stat.FileSize),
		Mode: fuse.S_IFREG | 0644,
		Mtime: uint64(f.stat.HeadTime),
	}, fuse.OK
}

func (f *p4File) fetch() bool {
	if f.backing != "" {
		return true
	}
	id := fmt.Sprintf("%s#%d", f.stat.DepotFile, f.stat.HeadRev)
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

	h := crypto.MD5.New()
	h.Write([]byte(id))
	dest := fmt.Sprintf("%s/%x", f.fs.backingDir, h.Sum(nil))
	os.Rename(tmp.Name(), dest)
	f.backing = dest
	return true
}

func (f *p4File) Deletable() bool {
	return false
}

func (n *p4File) Open(flags uint32, context *fuse.Context) (file fuse.File, code fuse.Status) {
	if flags & fuse.O_ANYWRITE != 0 {
		return nil, fuse.EROFS
	}
	
	n.fetch()
	f, err := os.OpenFile(n.backing, int(flags), 0644)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}
	return &fuse.LoopbackFile{File: f}, fuse.OK
}
