package dump

import (
	"unsafe"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"reflect"
	"context"
	"encoding/json"
)

var addrMapKey = 2020020002
var dumper = jsonfmt.Config{
	IncludesUnexported: true,
	Extensions: []jsonfmt.Extension{&dumpExtension{}},
}.Froze()

var efaceType = reflect.TypeOf(eface{})
var efaceEncoderInst = dumper.EncoderOf(reflect.TypeOf(eface{}))
var addrMapEncoderInst = jsonfmt.EncoderOf(reflect.TypeOf(map[string]json.RawMessage{}))

type Var struct {
	Object interface{}
}

func (v Var) String() string {
	addrMap := map[string]json.RawMessage{}
	ctx := context.WithValue(context.Background(), addrMapKey, addrMap)
	rootPtr := unsafe.Pointer(&v.Object)
	output := efaceEncoderInst.Encode(ctx, nil, rootPtr)
	addrMap["__root__"] = json.RawMessage(output)
	output = addrMapEncoderInst.Encode(nil, nil, reflect2.PtrOf(addrMap))
	return string(output)
}

type VarSnapshot string

func Snapshot(obj interface{}) VarSnapshot {
	return VarSnapshot(Var{Object: obj}.String())
}

func (snapshot VarSnapshot) String() string {
	return string(snapshot)
}

func ptrToStr(ptr uintptr) string {
	return string(jsonfmt.WriteUint64(nil, uint64(ptr)))
}

type dumpExtension struct {
}

func (extension *dumpExtension) EncoderOf(prefix string, valType reflect.Type) jsonfmt.Encoder {
	if valType == efaceType {
		return &efaceEncoder{}
	}
	switch valType.Kind() {
	case reflect.String:
		return &stringEncoder{}
	case reflect.Ptr:
		return &pointerEncoder{
			elemEncoder: dumper.EncoderOf(valType.Elem()),
		}
	case reflect.Slice:
		return &sliceEncoder{
			elemEncoder: dumper.EncoderOf(valType.Elem()),
			elemSize:    valType.Elem().Size(),
		}
	case reflect.Map:
		return newMapEncoder(dumper, valType)
	}
	return nil
}

type iface struct {
	itab unsafe.Pointer
	data unsafe.Pointer
}
