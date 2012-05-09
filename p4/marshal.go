package p4

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
)

var _ = log.Print

func decodeInt(r io.Reader) (int32, error) {
	var i [4]byte
	_, err := r.Read(i[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(i[:])), nil
}

var NoneObject interface{}

func init() {
	l := 1
	NoneObject = &l
}

func Decode(r io.Reader) (interface{}, error) {
	var t [1]byte
	_, err := r.Read(t[:])
	if err != nil {
		return nil, err
	}

	switch t[0] {
	case 'i':
		return decodeInt(r)
	case '0':
		return NoneObject, nil
	case '[':
		dest := []interface{}{}
		l, err := decodeInt(r)
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(l); i++ {
			v, err := Decode(r)
			if err != nil {
				return nil, err
			}
			dest = append(dest, v)
		}
		// list
	case '{':
		dest := make(map[interface{}]interface{})
		for {
			k, err := Decode(r)
			if err != nil {
				return nil, err
			}
			if k == NoneObject {
				return dest, nil
			}
			v, err := Decode(r)
			if err != nil {
				return nil, err
			}
			dest[k] = v
		}
	case 's':
		fallthrough
	case 'u':
		fallthrough
	case 't':
		// string
		l, err := decodeInt(r)
		if err != nil {
			return nil, err
		}

		s := make([]byte, l)
		_, err = r.Read(s)
		if err != nil {
			return nil, err
		}
		return string(s), nil
	case 'T':
		return true, nil
	case 'F':
		return false, nil
	}

	return nil, fmt.Errorf("unsupported type code %c", t[0])
}
