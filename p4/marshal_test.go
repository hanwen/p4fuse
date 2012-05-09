package p4

import (
	"bytes"
	"testing"
)

func TestMarshalInt(t *testing.T) {
	in := "i\x01\x00\x00\x00"
	out, err := Decode(bytes.NewBufferString(in))
	if err != nil {
		t.Fatalf("Decode err %v", err)
	}
	iout, ok := out.(int32)
	if !ok {
		t.Fatalf("want int32 got %T", out)
	}
	if iout != 1 {
		t.Fatalf("want 1 got %v", iout)
	}
}

func TestMarshalBool(t *testing.T) {
	in := "T"
	out, err := Decode(bytes.NewBufferString(in))
	if err != nil {
		t.Fatalf("Decode err %v", err)
	}
	iout, ok := out.(bool)
	if !ok {
		t.Fatalf("want bool got %T", out)
	}
	if iout != true {
		t.Fatalf("want true got %v", iout)
	}
}

func TestMarshalDict(t *testing.T) {
	in := "{i\x01\x00\x00\x00i\x01\x00\x00\x00i\x02\x00\x00\x00i\x02\x00\x00\x000"
	out, err := Decode(bytes.NewBufferString(in))
	if err != nil {
		t.Fatalf("Decode err %v", err)
	}
	iout, ok := out.(map[interface{}]interface{})
	if !ok {
		t.Fatalf("want dict got %T", out)
	}
	if len(iout) != 2 {
		t.Fatalf("want 2 entries %d", len(iout))
	}
}

func TestMarshalStr(t *testing.T) {
	in := "t\x05\x00\x00\x00hello"
	out, err := Decode(bytes.NewBufferString(in))
	if err != nil {
		t.Fatalf("Decode err %v", err)
	}
	iout, ok := out.(string)
	if !ok {
		t.Fatalf("want string got %T", out)
	}
	if iout != "hello" {
		t.Fatalf("want 1 got %q", iout)
	}
}

func xTestMarshal(t *testing.T) {
	//marshal.dumps({1:-1, 'key':'val', True: [1,2,3,]})
	in := "{i\x01\x00\x00\x00[\x03\x00\x00\x00i\x01\x00\x00\x00i\x02\x00\x00\x00i\x03\x00\x00\x00t\x03\x00\x00\x00keyt\x03\x00\x00\x00val0"
	out, err := Decode(bytes.NewBufferString(in))
	if err != nil {
		t.Fatalf("Decode err %v", err)
	}

	dict := out.(map[interface{}]interface{})
	if len(dict) != 3 {
		t.Errorf("dict was %d, want 3", len(dict))
	}
}
