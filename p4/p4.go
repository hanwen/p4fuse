// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p4

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

// Conn is an interface to the Conn command line client.

type ConnOptions struct {
	Address string
	Binary  string
}

type Conn struct {
	opts *ConnOptions
}

func NewConn(opts ConnOptions) *Conn {
	return &Conn{&opts}
}

type TagLine struct {
	Tag   string
	Value []byte
}

// Output runs p4 and captures stdout.
func (p *Conn) Output(args []string) ([]byte, error) {
	b := p.opts.Binary
	if !strings.Contains(b, "/") {
		b, _ = exec.LookPath(b)
	}
	cmd := exec.Cmd{
		Path: b,
		Args: []string{p.opts.Binary},
	}
	if p.opts.Address != "" {
		cmd.Args = append(cmd.Args, "-p", p.opts.Address)
	}
	cmd.Args = append(cmd.Args, args...)

	log.Println("running", cmd.Args)
	return cmd.Output()
}

// Runs p4 with -G and captures the result lines.
func (p *Conn) RunMarshaled(command string, args []string) (result []Result, err error) {
	out, err := p.Output(append([]string{"-G", command}, args...))
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
		result = append(result, interpretResult(asMap, command))
	}

	if len(result) > 0 {
		err = nil
	}

	return result, err
}

func interpretResult(in map[interface{}]interface{}, command string) Result {
	imap := map[string]interface{}{}
	for k, v := range in {
		imap[k.(string)] = v
	}
	code := imap["code"].(string)
	if code == "error" {
		e := Error{}
		e.Severity = int(imap["severity"].(int32))
		e.Generic = int(imap["generic"].(int32))
		e.Data = imap["data"].(string)
		return &e
	}

	switch command {
	case "dirs":
		return &Dir{Dir: imap["dir"].(string)}
	case "fstat":
		r := map[string]string{}
		for k, v := range imap {
			r[k] = v.(string)
		}

		st := Stat{
			DepotFile:  r["depotFile"],
			HeadAction: r["headAction"],
			Digest:     r["digest"],
			HeadType:   r["headType"],
		}

		// Brilliant. We get the integers as decimal strings. Sigh.
		st.HeadTime, _ = strconv.ParseInt(r["headTime"], 10, 64)
		st.HeadRev, _ = strconv.ParseInt(r["headRev"], 10, 64)
		st.HeadChange, _ = strconv.ParseInt(r["headChange"], 10, 64)
		st.HeadModTime, _ = strconv.ParseInt(r["headModTime"], 10, 64)
		st.FileSize, _ = strconv.ParseInt(r["fileSize"], 10, 64)
		return &st

	case "changes":
		r := map[string]string{}
		for k, v := range imap {
			r[k] = v.(string)
		}
		c := Change{
			Desc:       r["desc"],
			User:       r["user"],
			Status:     r["status"],
			Path:       r["path"],
			Code:       r["code"],
			ChangeType: r["changeType"],
			Client:     r["client"],
		}
		cl, _ := strconv.ParseInt(r["change"], 10, 64)
		c.Change = int(cl)
		t, _ := strconv.ParseInt(r["time"], 10, 64)
		c.Time = int(t)
		return &c
	default:
		log.Panicf("unknown code %q", command)
	}
	return nil
}

func (p *Conn) Fstat(paths []string) (results []Result, err error) {
	r, err := p.RunMarshaled("fstat",
		append([]string{"-Ol"}, paths...))
	return r, err
}

func (p *Conn) Dirs(paths []string) ([]Result, error) {
	return p.RunMarshaled("dirs", paths)
}

func (p *Conn) Print(path string) (content []byte, err error) {
	out, err := p.Output([]string{"print", path})
	if err != nil {
		return nil, err
	}
	parts := bytes.SplitN(out, []byte{'\n'}, 2)
	return parts[1], nil
}

func (p *Conn) Changes(paths []string) ([]Result, error) {
	return p.RunMarshaled("changes", append([]string{"-l"}, paths...))
}

////////////////
type Result interface {
	String() string
}

type Error struct {
	Generic  int
	Severity int
	Data     string
}

func (e *Error) String() string {
	return fmt.Sprintf("error %d(%d): %s", e.Generic, e.Severity, e.Data)
}

// Stat has the data for a single file revision.
type Stat struct {
	DepotFile   string
	HeadAction  string
	HeadType    string
	HeadTime    int64
	HeadRev     int64
	HeadChange  int64
	HeadModTime int64
	FileSize    int64
	Digest      string
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

type Change struct {
	Desc   string
	User   string
	Status string
	Change int
	Time   int

	Path       string
	Code       string
	ChangeType string
	Client     string
}

func (c *Change) String() string {
	l := len(c.Desc)
	if l > 250 {
		l = 250
	}
	return fmt.Sprintf("change %d by %s - %s", c.Change, c.User, strings.Trim(c.Desc[:l], " "))
}
