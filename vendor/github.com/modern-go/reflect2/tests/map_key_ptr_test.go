package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
	"github.com/modern-go/test/must"
	"github.com/modern-go/test"

	"unsafe"
	"context"
)

func Test_map_key_ptr(t *testing.T) {
	var pInt = func(val int) *int {
		return &val
	}
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[*int]int{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		key := pInt(2)
		valType.SetIndex(obj, &key, 4)
		valType.SetIndex(obj, &key, 9)
		//valType.SetIndex(obj, nil, 9)
		return obj[pInt(2)]
	}))
	t.Run("UnsafeSetIndex", test.Case(func(ctx context.Context) {
		obj := map[*int]int{}
		valType := reflect2.TypeOf(obj).(reflect2.MapType)
		v := pInt(2)
		valType.UnsafeSetIndex(reflect2.PtrOf(obj), unsafe.Pointer(v), reflect2.PtrOf(4))
		must.Equal(4, obj[v])
	}))
	t.Run("GetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[*int]int{pInt(3): 9, pInt(2): 4}
		valType := api.TypeOf(obj).(reflect2.MapType)
		return []interface{}{
			valType.GetIndex(obj, pInt(3)),
			valType.GetIndex(obj, pInt(2)),
			valType.GetIndex(obj, nil),
		}
	}))
	t.Run("Iterate", testOp(func(api reflect2.API) interface{} {
		obj := map[*int]int{pInt(2): 4}
		valType := api.TypeOf(obj).(reflect2.MapType)
		iter := valType.Iterate(&obj)
		must.Pass(iter.HasNext(), "api", api)
		key1, elem1 := iter.Next()
		must.Pass(!iter.HasNext(), "api", api)
		return []interface{}{key1, elem1}
	}))
}
