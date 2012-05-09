// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os/exec"
	"log"
	"io"
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
func (p *P4) RunMarshaled(args []string) (result []Result, err error) {
	out, err := p.Output(append([]string{"-G"}, args...))
	r := bytes.NewBuffer(out)
	for {
		v, err := Decode(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		asMap, ok := v.(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("format err: p4 marshaled %v", v)
		}
		result = append(result, interpretResult(asMap))
	}

	if len(result) > 0 {
		err = nil
	}
	
	return result, err
}
	
func interpretResult(in map[interface{}]interface{}) Result {
	imap := map[string]interface{}{}
	for k, v := range in {
		imap[k.(string)] = v
	}
	code  := imap["code"].(string)
	switch code {
	case "stat":
		r := map[string]string{}
		for k, v := range imap {
			r[k] = v.(string)
		}

		if r["dir"] != "" {
			return &Dir{Dir: r["dir"]}
		}
		st := Stat{}
		st.DepotFile = r["depotFile"]
		st.HeadAction = r["headAction"]
		st.Digest = r["digest"]
		st.HeadType = r["headType"]
		st.HeadTime, _ = strconv.ParseInt(r["headTime"], 10, 64)
		st.HeadRev, _ = strconv.ParseInt(r["headRev"], 10, 64)
		st.HeadChange, _ = strconv.ParseInt(r["headChange"], 10, 64)
		st.HeadModTime, _ = strconv.ParseInt(r["headModTime"], 10, 64)
		st.FileSize, _ = strconv.ParseInt(r["fileSize"], 10, 64)
		return &st
	case "error":
		e := Error{}
		e.Severity = int(imap["severity"].(int32))
		e.Generic = int(imap["generic"].(int32))
		e.Data = imap["data"].(string)
		return &e
	default:
		log.Panicf("unknown code %q", code)
	}
	return nil
}

func (p *P4) Fstat(paths []string) (results []Result, err error) {
	r, err := p.RunMarshaled(
		append([]string{"fstat", "-Ol"}, paths...))
	return r, err
}

func (p *P4) Dirs(paths []string) ([]Result, error) {
	return p.RunMarshaled(
		append([]string{"dirs"}, paths...))
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
type Result interface {
	String() string
}
	
type Error struct {
	Generic int
	Severity int
	Data string
}

func (e *Error) String() string {
	return fmt.Sprintf("error %d(%d): %s", e.Generic, e.Severity, e.Data)
}	

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

type Dir struct {
	Dir string
}

func (f *Dir) String() string {
	return fmt.Sprintf("%s/", f.Dir)
}
