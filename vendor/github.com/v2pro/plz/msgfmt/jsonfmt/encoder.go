package jsonfmt

import (
	"unsafe"
	"reflect"
	"strings"
	"unicode"
	"fmt"
	"encoding/json"
	"context"
	"github.com/v2pro/plz/reflect2"
	"github.com/v2pro/plz/concurrent"
)

var bytesType = reflect2.TypeOf([]byte(nil))
var errorType = reflect2.TypeOfPtr((*error)(nil)).Elem()
var jsonMarshalerType = reflect2.TypeOfPtr((*json.Marshaler)(nil)).Elem()

type Encoder interface {
	Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte
}

type Extension interface {
	EncoderOf(prefix string, valType reflect.Type) Encoder
}

type Config struct {
	IncludesUnexported bool
	Extensions         []Extension
}

type API interface {
	EncoderOf(valType reflect2.Type) Encoder
	EncoderOfObject(obj interface{}) Encoder
}

var ConfigDefault = Config{}.Froze()


type frozenConfig struct {
	includesUnexported bool
	extensions         []Extension
	encoderCache       *concurrent.Map
	mapKeyEncoderCache *concurrent.Map
}

func (cfg Config) Froze() API {
	return &frozenConfig{
		includesUnexported: cfg.IncludesUnexported,
		extensions:         cfg.Extensions,
		encoderCache:       concurrent.NewMap(),
		mapKeyEncoderCache: concurrent.NewMap(),
	}
}

func (cfg *frozenConfig) EncoderOfObject(obj interface{}) Encoder {
	cacheKey := reflect2.RTypeOf(obj)
	encoderObj, found := cfg.encoderCache.Load(cacheKey)
	if found {
		return encoderObj.(Encoder)
	}
	return cfg.EncoderOf(reflect2.TypeOf(obj))
}

func (cfg *frozenConfig) EncoderOf(valType reflect2.Type) Encoder {
	cacheKey := valType.RType()
	encoderObj, found := cfg.encoderCache.Load(cacheKey)
	if found {
		return encoderObj.(Encoder)
	}
	encoder := encoderOf(cfg, "", valType)
	if valType.LikePtr() {
		encoder = &onePtrInterfaceEncoder{encoder}
	}
	cfg.encoderCache.Store(cacheKey, encoder)
	return encoder
}

func encoderOfMapKey(cfg *frozenConfig, prefix string, keyType reflect2.Type) Encoder {
	cacheKey := keyType.RType()
	encoderObj, found := cfg.mapKeyEncoderCache.Load(cacheKey)
	if found {
		return encoderObj.(Encoder)
	}
	encoder := _encoderOfMapKey(cfg, prefix, keyType)
	cfg.mapKeyEncoderCache.Store(cacheKey, encoder)
	return encoder
}

func EncoderOf(valType reflect2.Type) Encoder {
	return ConfigDefault.EncoderOf(valType)
}

func EncoderOfObject(obj interface{}) Encoder {
	return ConfigDefault.EncoderOfObject(obj)
}

func MarshalToString(obj interface{}) string {
	encoder := EncoderOfObject(obj)
	return string(encoder.Encode(nil, nil, reflect2.PtrOf(obj)))
}

func encoderOf(cfg *frozenConfig, prefix string, valType reflect2.Type) Encoder {
	for _, extension := range cfg.extensions {
		encoder := extension.EncoderOf(prefix, valType.Type1())
		if encoder != nil {
			return encoder
		}
	}
	if bytesType == valType {
		return &bytesEncoder{}
	}
	if valType.Implements(errorType) {
		return &errorEncoder{
			valType: valType,
		}
	}
	if valType.Implements(jsonMarshalerType) {
		return &jsonMarshalerEncoder{
			valType: valType,
		}
	}
	switch valType.Kind() {
	case reflect.Bool:
		return &boolEncoder{}
	case reflect.Int8:
		return &int8Encoder{}
	case reflect.Uint8:
		return &uint8Encoder{}
	case reflect.Int16:
		return &int16Encoder{}
	case reflect.Uint16:
		return &uint16Encoder{}
	case reflect.Int32:
		return &int32Encoder{}
	case reflect.Uint32:
		return &uint32Encoder{}
	case reflect.Int64, reflect.Int:
		return &int64Encoder{}
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		return &uint64Encoder{}
	case reflect.Float64:
		return &lossyFloat64Encoder{}
	case reflect.Float32:
		return &lossyFloat32Encoder{}
	case reflect.String:
		return &stringEncoder{}
	case reflect.Ptr:
		pointerType := valType.(reflect2.PtrType)
		elemEncoder := encoderOf(cfg, prefix+" [ptrElem]", pointerType.Elem())
		return &pointerEncoder{elemEncoder: elemEncoder}
	case reflect.Slice:
		sliceType := valType.(reflect2.SliceType)
		elemEncoder := encoderOf(cfg, prefix+" [sliceElem]", sliceType.Elem())
		return &sliceEncoder{
			elemEncoder: elemEncoder,
			sliceType:   sliceType.(*reflect2.UnsafeSliceType),
		}
	case reflect.Array:
		arrayType := valType.(reflect2.ArrayType)
		elemEncoder := encoderOf(cfg, prefix+" [sliceElem]", arrayType.Elem())
		return &arrayEncoder{
			elemEncoder: elemEncoder,
			arrayType:   arrayType.(*reflect2.UnsafeArrayType),
		}
	case reflect.Struct:
		structType := valType.(reflect2.StructType)
		return encoderOfStruct(cfg, prefix, structType)
	case reflect.Map:
		mapType := valType.(reflect2.MapType)
		return encoderOfMap(cfg, prefix, mapType)
	case reflect.Interface:
		return &dynamicEncoder{valType:valType}
	}
	return &unsupportedEncoder{fmt.Sprintf(`"can not encode %s %s to json"`, valType.String(), prefix)}
}

type unsupportedEncoder struct {
	msg string
}

func (encoder *unsupportedEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return append(space, encoder.msg...)
}

func encoderOfMap(cfg *frozenConfig, prefix string, valType reflect2.MapType) *mapEncoder {
	keyEncoder := encoderOfMapKey(cfg, prefix, valType.Key())
	elemType := valType.Elem()
	elemEncoder := encoderOf(cfg, prefix+" [mapElem]", elemType)
	return &mapEncoder{
		keyEncoder:  keyEncoder,
		elemEncoder: elemEncoder,
		mapType:     valType.(*reflect2.UnsafeMapType),
	}
}

func _encoderOfMapKey(cfg *frozenConfig, prefix string, keyType reflect2.Type) Encoder {
	keyEncoder := encoderOf(cfg, prefix+" [mapKey]", keyType)
	if keyType.Kind() == reflect.String || keyType == bytesType {
		return &mapStringKeyEncoder{keyEncoder}
	}
	if keyType.Kind() == reflect.Interface {
		return &mapInterfaceKeyEncoder{cfg: cfg, prefix: prefix}
	}
	return &mapNumberKeyEncoder{keyEncoder}
}

type onePtrInterfaceEncoder struct {
	valEncoder Encoder
}

func (encoder *onePtrInterfaceEncoder) Encode(ctx context.Context, space []byte, ptr unsafe.Pointer) []byte {
	return encoder.valEncoder.Encode(ctx, space, unsafe.Pointer(&ptr))
}

func encoderOfStruct(cfg *frozenConfig, prefix string, valType reflect2.StructType) *structEncoder {
	var fields []structEncoderField
	for i := 0; i < valType.NumField(); i++ {
		field := valType.Field(i)
		name := getFieldName(cfg, field)
		if name == "" {
			continue
		}
		prefix := ""
		if len(fields) != 0 {
			prefix += ","
		}
		prefix += `"`
		prefix += name
		prefix += `":`
		fields = append(fields, structEncoderField{
			structField: field.(*reflect2.UnsafeStructField),
			prefix:      prefix,
			encoder:     encoderOf(cfg, prefix+" ."+name, field.Type()),
		})
	}
	return &structEncoder{
		fields: fields,
	}
}

func getFieldName(cfg *frozenConfig, field reflect2.StructField) string {
	if !cfg.includesUnexported && !unicode.IsUpper(rune(field.Name()[0])) {
		return ""
	}
	if field.Type().Kind() == reflect.Func {
		return ""
	}
	if field.Type().Kind() == reflect.Chan {
		return ""
	}
	jsonTag := field.Tag().Get("json")
	if jsonTag == "" {
		return field.Name()
	}
	parts := strings.Split(jsonTag, ",")
	if parts[0] == "-" {
		return ""
	}
	if parts[0] == "" {
		return field.Name()
	}
	return parts[0]
}