package jsonfmt

import (
	"unsafe"
	"context"
	"github.com/v2pro/plz/reflect2"
)

type mapEncoder struct {
	keyEncoder  Encoder
	elemEncoder Encoder
	mapType     *reflect2.UnsafeMapType
}

func (encoder *mapEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	if *(*unsafe.Pointer)(ptr) == nil {
		return append(space, 'n', 'u', 'l', 'l')
	}
	iter := encoder.mapType.UnsafeIterate(ptr)
	space = append(space, '{')
	for i := 0; iter.HasNext(); i++ {
		if i != 0 {
			space = append(space, ',')
		}
		key, elem := iter.UnsafeNext()
		space = encoder.keyEncoder.Encode(ctx, space, key)
		space = encoder.elemEncoder.Encode(ctx, space, elem)
	}
	space = append(space, '}')
	return space
}

type mapNumberKeyEncoder struct {
	valEncoder Encoder
}

func (encoder *mapNumberKeyEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	space = append(space, '"')
	space = encoder.valEncoder.Encode(ctx, space, ptr)
	space = append(space, '"', ':')
	return space
}

type mapStringKeyEncoder struct {
	valEncoder Encoder
}

func (encoder *mapStringKeyEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	space = encoder.valEncoder.Encode(ctx, space, ptr)
	space = append(space, ':')
	return space
}

type mapInterfaceKeyEncoder struct {
	cfg    *frozenConfig
	prefix string
}

func (encoder *mapInterfaceKeyEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	keyObj := *(*interface{})(ptr)
	keyEncoder := encoderOfMapKey(encoder.cfg, encoder.prefix, reflect2.TypeOf(keyObj))
	return keyEncoder.Encode(ctx, space, reflect2.PtrOf(keyObj))
}
