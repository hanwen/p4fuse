// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"github.com/hanwen/go-fuse/fuse"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	fsdebug := flag.Bool("fs-debug", false, "switch on FS debugging")
	p4port := flag.String("p4-server", "localhost:1492", "address for P4 server")
	p4binary := flag.String("p4-binary", "p4", "binary for P4 commandline client")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatal("Usage: p4fs MOUNT-POINT")
	}
	mountpoint := flag.Arg(0)

	p4 := &P4{Binary: *p4binary, Address: *p4port}
	backingDir, err := ioutil.TempDir("", "p4fs")
	if err != nil {
		log.Fatalf("TempDir failed: %v", err)
	}
	fs := NewP4Fs(p4, backingDir)
	conn := fuse.NewFileSystemConnector(fs, fuse.NewFileSystemOptions())
	rawFs := fuse.NewLockingRawFileSystem(conn)

	mount := fuse.NewMountState(rawFs)
	if err := mount.Mount(mountpoint, nil); err != nil {
		log.Fatalf("mount failed: %v", err)
	}

	conn.Debug = *fsdebug
	mount.Debug = *fsdebug
	log.Println("starting FUSE.")
	mount.Loop()
	os.RemoveAll(backingDir)
}



