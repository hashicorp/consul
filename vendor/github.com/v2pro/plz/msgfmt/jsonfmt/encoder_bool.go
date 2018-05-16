package jsonfmt

import (
	"context"
	"unsafe"
)

type boolEncoder struct {
}

func (encoder *boolEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	if *(*bool)(ptr) {
		space = append(space, 't', 'r', 'u', 'e')
	} else {
		space = append(space, 'f', 'a', 'l', 's', 'e')
	}
	return space
}