package p4

import (
	"testing"
)

// Assumes sample depot is running on localhost:1666, and p4 binary is in path.
func TestDirs(t *testing.T) {
	c := NewConn()
	rs, err := c.Dirs([]string{"//depot/*@700"})
	if err != nil {
		t.Fatalf("p4.Dirs: %v", err)
	}

	if len(rs) != 1 {
		t.Fatalf("p4.Dirs got: %v, want 1 result", rs)
	}

	d := rs[0].(*Dir)
	if d.Dir != "//depot/Jam" {
		t.Fatalf("p4.Dirs got dir %q", d.Dir)
	}
}
