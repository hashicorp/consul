package jsonfmt

import (
	"unsafe"
	"encoding/json"
	"context"
	"github.com/v2pro/plz/reflect2"
)

type jsonMarshalerEncoder struct {
	valType reflect2.Type
}

func (encoder *jsonMarshalerEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	obj := encoder.valType.UnsafeIndirect(ptr)
	buf, err := obj.(json.Marshaler).MarshalJSON()
	if err != nil {
		space = append(space, '"')
		space = append(space, err.Error()...)
		space = append(space, '"')
		return space
	}
	space = append(space, buf...)
	return space
}