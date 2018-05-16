package jsonfmt

import (
	"unsafe"
	"context"
	"github.com/v2pro/plz/reflect2"
)

type arrayEncoder struct {
	elemEncoder Encoder
	arrayType   *reflect2.UnsafeArrayType
}

func (encoder *arrayEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	space = append(space, '[')
	arrayType := encoder.arrayType
	for i := 0; i < arrayType.Len(); i++ {
		if i != 0 {
			space = append(space, ',')
		}
		elemPtr := arrayType.UnsafeGetIndex(ptr, i)
		space = encoder.elemEncoder.Encode(ctx, space, elemPtr)
	}
	space = append(space, ']')
	return space
}
