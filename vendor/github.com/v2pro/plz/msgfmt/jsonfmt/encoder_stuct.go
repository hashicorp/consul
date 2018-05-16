package jsonfmt

import (
	"unsafe"
	"context"
	"github.com/v2pro/plz/reflect2"
)

type structEncoder struct {
	fields []structEncoderField
}

type structEncoderField struct {
	structField *reflect2.UnsafeStructField
	prefix  string
	encoder Encoder
}

func (encoder *structEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	space = append(space, '{')
	for _, field := range encoder.fields {
		space = append(space, field.prefix...)
		fieldPtr := field.structField.UnsafeGet(ptr)
		space = field.encoder.Encode(ctx, space, fieldPtr)
	}
	space = append(space, '}')
	return space
}
