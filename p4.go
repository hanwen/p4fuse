// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os/exec"
	"log"
	"bytes"
	"fmt"
	"strconv"
)

// P4 is an interface to the P4 command line client. 
type P4 struct {
	Address string
	Binary  string
}

type TagLine struct {
	Tag   string
	Value []byte
}

// Output runs p4 and captures stdout.
func (p *P4) Output(args []string) ([]byte, error) {
	cmd := exec.Cmd{
		Path: p.Binary,
		Args: append([]string{p.Binary, "-p", p.Address}, args...),
	}
	log.Println("running", cmd.Args)
	return cmd.Output()
}

// Runs p4 with -zTAG -s and captures the result lines.
func (p *P4) RunTagged(args []string) (result []TagLine, err error) {
	out, err := p.Output(append([]string{"-s", "-Ztag"}, args...))

	lines := bytes.Split(out, []byte{'\n'})
	for _, ln := range lines {
		idx := bytes.IndexByte(ln, ':')
		if idx < 0 {
			return nil, fmt.Errorf("format error %q", ln) 
		}
		tag := string(ln[:idx])
		val := ln[idx+2:]
		result = append(result, TagLine{
			Tag: tag,
			Value: val,
		})
		if tag == "exit" {
			// Don't return error; the invocation was
			// successful.
			return result, nil
		}
	}
	return nil, err
}

func (p *P4) Fstat(paths []string) (stats map[string]*Stat, err error) {
	stats = map[string]*Stat{}
	res, err := p.RunTagged(
		append([]string{"fstat", "-Ol"}, paths...))
	if err != nil {
		return nil, err
	}
	st := &Stat{}
	for _, r := range res {
		if r.Tag != "info1" {
			continue
		}
		parts := bytes.SplitN(r.Value, []byte{' '}, 2)
		if len(parts) != 2 {
			log.Printf("format error: %q", r.Value)
			continue
		}
			
		key := string(parts[0])
		val := string(parts[1])

		intVal, _ := strconv.ParseInt(val, 10, 64)
		switch key {
		case "depotFile":
			if st.DepotFile != "" {
				stats[st.DepotFile] = st
			}
			st = &Stat{}
			st.DepotFile = val
		case "headAction":
			st.HeadAction = val
		case "headType":
			st.HeadType = val
		case "headTime":
			st.HeadTime = intVal
		case "headRev":
			st.HeadRev = intVal
		case "headChange":
			st.HeadChange = intVal
		case "headModTime":
			st.HeadModTime = intVal
		case "fileSize":
			st.FileSize = intVal
		case "digest":
			st.Digest = val
		default:
			log.Printf("ignoring unknown Stat key %q", key)
		}
	}

	return stats, nil
}

func (p *P4) Dirs(paths []string) (dirs []string, err error) {
	res, err := p.RunTagged(
		append([]string{"dirs"}, paths...))
	if err != nil {
		return nil, err
	}
	for _, r := range res {
		parts := bytes.SplitN(r.Value, []byte{' '}, 2)
		if string(parts[0]) == "dir" {
			// what happens if path has a \n ?
			dirs = append(dirs, string(parts[1]))
		}
	}
	return dirs, nil
}

func (p *P4) Print(path string) (content []byte, err error) {
	out, err := p.Output([]string{"print", path})
	if err != nil {
		return nil, err
	}
	parts := bytes.SplitN(out, []byte{'\n'}, 2)
	return parts[1], nil
}

////////////////
// Stat has the data for a single file revision.
type Stat struct {
	DepotFile string
	HeadAction string
	HeadType string
	HeadTime int64
	HeadRev int64
	HeadChange int64
	HeadModTime int64
	FileSize int64
	Digest string
}

func (f *Stat) String() string {
	return fmt.Sprintf("%s#%d - change %d (%s)",
		f.DepotFile, f.HeadRev, f.HeadChange, f.HeadType)
}
