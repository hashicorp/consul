package jsonfmt

import (
	"unsafe"
	"context"
	"github.com/v2pro/plz/reflect2"
)

type dynamicEncoder struct {
	valType reflect2.Type
}

func (encoder *dynamicEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	obj := encoder.valType.UnsafeIndirect(ptr)
	if obj == nil {
		return append(space, 'n', 'u', 'l', 'l')
	}
	return EncoderOf(reflect2.TypeOf(obj)).Encode(ctx, space, reflect2.PtrOf(obj))
}