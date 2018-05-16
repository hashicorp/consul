package dump

import (
	"context"
	"unsafe"
	"reflect"
	"encoding/json"
)

type eface struct {
	dataType unsafe.Pointer
	data     unsafe.Pointer
}

var sampleType = reflect.TypeOf("")

type efaceEncoder struct {
}

func (encoder *efaceEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	space = append(space, `{"type":"`...)
	eface := (*eface)(ptr)
	valType := sampleType
	(*iface)(unsafe.Pointer(&valType)).data = eface.dataType
	space = append(space, valType.String()...)
	space = append(space, `","data":{"__ptr__":"`...)
	ptrStr := ptrToStr(uintptr(eface.data))
	space = append(space, ptrStr...)
	space = append(space, `"}}`...)
	elemEncoder := dumper.EncoderOf(valType)
	elem := elemEncoder.Encode(ctx, nil, eface.data)
	addrMap := ctx.Value(addrMapKey).(map[string]json.RawMessage)
	addrMap[ptrStr] = json.RawMessage(elem)
	return space
}