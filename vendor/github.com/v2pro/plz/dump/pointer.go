package dump

import (
	"context"
	"unsafe"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"encoding/json"
)

type pointerEncoder struct {
	elemEncoder jsonfmt.Encoder
}

func (encoder *pointerEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	space = append(space, `{"__ptr__":"`...)
	ptrStr := ptrToStr(uintptr(ptr))
	space = append(space, ptrStr...)
	space = append(space, `"}`...)
	elem := encoder.elemEncoder.Encode(ctx, nil, *(*unsafe.Pointer)(ptr))
	addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
	addrMap[ptrStr] = json.RawMessage(elem)
	return space
}