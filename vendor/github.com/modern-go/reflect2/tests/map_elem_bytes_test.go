package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
	"github.com/modern-go/test"
	"github.com/modern-go/test/must"
	"context"
)

func Test_map_elem_bytes(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[int][]byte{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, []byte("hello"))
		valType.SetIndex(obj, 3, nil)
		return obj
	}))
	t.Run("UnsafeSetIndex", test.Case(func(ctx context.Context) {
		obj := map[int][]byte{}
		valType := reflect2.TypeOf(obj).(reflect2.MapType)
		hello := []byte("hello")
		valType.UnsafeSetIndex(reflect2.PtrOf(obj), reflect2.PtrOf(2), reflect2.PtrOf(hello))
		valType.UnsafeSetIndex(reflect2.PtrOf(obj), reflect2.PtrOf(3), nil)
		must.Equal([]byte("hello"), obj[2])
		must.Nil(obj[3])
	}))
	t.Run("UnsafeGetIndex", test.Case(func(ctx context.Context) {
		obj := map[int][]byte{2: []byte("hello")}
		valType := reflect2.TypeOf(obj).(reflect2.MapType)
		elem := valType.UnsafeGetIndex(reflect2.PtrOf(obj), reflect2.PtrOf(2))
		must.Equal([]byte("hello"), valType.Elem().PackEFace(elem))
	}))
}
