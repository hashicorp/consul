package jsonfmt

import (
	"unsafe"
	"context"
	"github.com/v2pro/plz/reflect2"
)

type errorEncoder struct {
	valType reflect2.Type
}

func (encoder *errorEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	obj := encoder.valType.UnsafeIndirect(ptr)
	space = append(space, '"')
	space = append(space, obj.(error).Error()...)
	space = append(space, '"')
	return space
}
