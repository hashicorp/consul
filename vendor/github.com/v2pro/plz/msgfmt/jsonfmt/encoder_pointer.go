package jsonfmt

import (
	"unsafe"
	"context"
)

type pointerEncoder struct {
	elemEncoder Encoder
}

func (encoder *pointerEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	ptrTo := *(*unsafe.Pointer)(ptr)
	if ptrTo == nil {
		return append(space, 'n', 'u', 'l', 'l')
	}
	return encoder.elemEncoder.Encode(ctx, space, ptrTo)
}
