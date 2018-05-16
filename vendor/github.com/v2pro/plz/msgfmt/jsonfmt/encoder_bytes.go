package jsonfmt

import (
	"unsafe"
	"unicode/utf8"
	"context"
)

type bytesEncoder struct {
}

func (encoder *bytesEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return WriteBytes(space, *(*[]byte)(ptr))
}

func WriteBytes(space []byte, s []byte) []byte {
	space = append(space, '"')
	// write string, the fast path, without utf8 and escape support
	var i int
	var c byte
	for i, c = range s {
		if c < utf8.RuneSelf && safeSet[c] {
			space = append(space, c)
		} else {
			break
		}
	}
	if i == len(s)-1 {
		space = append(space, '"')
		return space
	}
	return writeBytesSlowPath(space, s[i:])
}

func writeBytesSlowPath(space []byte, s []byte) []byte {
	start := 0
	// for the remaining parts, we process them char by char
	var i int
	var b byte
	for i, b = range s {
		if b >= utf8.RuneSelf {
			space = append(space, '\\', '\\', 'x', hex[b>>4], hex[b&0xF])
			start = i + 1
			continue
		}
		if safeSet[b] {
			continue
		}
		if start < i {
			space = append(space, s[start:i]...)
		}
		switch b {
		case '\\', '"':
			space = append(space, '\\', b)
		case '\n':
			space = append(space, '\\', 'n')
		case '\r':
			space = append(space, '\\', 'r')
		case '\t':
			space = append(space, '\\', 't')
		default:
			// This encodes bytes < 0x20 except for \t, \n and \r.
			space = append(space, '\\', '\\', 'x', hex[b>>4], hex[b&0xF])
		}
		start = i + 1
	}
	if start < len(s) {
		space = append(space, s[start:]...)
	}
	return append(space, '"')
}