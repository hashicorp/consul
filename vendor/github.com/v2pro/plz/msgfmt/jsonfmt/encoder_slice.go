package jsonfmt

import (
	"unsafe"
	"context"
	"github.com/v2pro/plz/reflect2"
)

type sliceEncoder struct {
	elemEncoder Encoder
	sliceType   *reflect2.UnsafeSliceType
}

func (encoder *sliceEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	sliceType := encoder.sliceType
	space = append(space, '[')
	length := sliceType.UnsafeLengthOf(ptr)
	for i := 0; i < length; i++ {
		if i != 0 {
			space = append(space, ',')
		}
		elemPtr := sliceType.UnsafeGetIndex(ptr, i)
		space = encoder.elemEncoder.Encode(ctx, space, elemPtr)
	}
	space = append(space, ']')
	return space
}
