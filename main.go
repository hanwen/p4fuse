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
	"runtime/pprof"

	"p4fuse/p4"
)

func main() {
	fsdebug := flag.Bool("fs-debug", false, "switch on FS debugging")
	p4port := flag.String("p4-server", "", "address for P4 server")
	p4binary := flag.String("p4-binary", "p4", "binary for P4 commandline client")
	backingDir := flag.String("backing", "", "directory to store file contents.")
	profile := flag.String("profile", "", "record cpu profile.")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatal("Usage: p4fs MOUNT-POINT")
	}
	mountpoint := flag.Arg(0)

	opts := p4.ConnOptions{
		Binary:  *p4binary,
		Address: *p4port,
	}
	p4conn := p4.NewConn(opts)

	if *backingDir == "" {
		d, err := ioutil.TempDir("", "p4fs")
		if err != nil {
			log.Fatalf("TempDir failed: %v", err)
		}
		*backingDir = d
		defer os.RemoveAll(d)
	}

	fs := NewP4Fs(p4conn, *backingDir)
	conn := fuse.NewFileSystemConnector(fs, fuse.NewFileSystemOptions())

	mount := fuse.NewMountState(conn)
	if err := mount.Mount(mountpoint, nil); err != nil {
		log.Fatalf("mount failed: %v", err)
	}

	conn.Debug = *fsdebug
	mount.Debug = *fsdebug
	log.Println("starting FUSE.")

	if *profile != "" {
		profFile, err := os.Create(*profile)
		if err != nil {
			log.Fatalf("os.Create: %v", err)
		}
		pprof.StartCPUProfile(profFile)
		defer pprof.StopCPUProfile()
	}

	mount.Loop()
}
