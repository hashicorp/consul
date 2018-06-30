package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
	"github.com/modern-go/test/must"

	"github.com/modern-go/test"
	"context"
)

func Test_map_elem_eface(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[int]interface{}{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, 4)
		valType.SetIndex(obj, 3, nil)
		return obj
	}))
	t.Run("GetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[int]interface{}{3: 9, 2: nil}
		valType := api.TypeOf(obj).(reflect2.MapType)
		return []interface{}{
			valType.GetIndex(obj, 3),
			valType.GetIndex(obj, 2),
			valType.GetIndex(obj, 0),
		}
	}))
	t.Run("TryGetIndex", test.Case(func(ctx context.Context) {
		obj := map[int]interface{}{3: 9, 2: nil}
		valType := reflect2.TypeOf(obj).(reflect2.MapType)
		elem, found := valType.TryGetIndex(obj, 3)
		must.Equal(9, elem)
		must.Pass(found)
		elem, found = valType.TryGetIndex(obj, 2)
		must.Nil(elem)
		must.Pass(found)
		elem, found = valType.TryGetIndex(obj, 0)
		must.Nil(elem)
		must.Pass(!found)
	}))
	t.Run("Iterate", testOp(func(api reflect2.API) interface{} {
		obj := map[int]interface{}{2: 4}
		valType := api.TypeOf(obj).(reflect2.MapType)
		iter := valType.Iterate(obj)
		must.Pass(iter.HasNext(), "api", api)
		key1, elem1 := iter.Next()
		must.Pass(!iter.HasNext(), "api", api)
		return []interface{}{key1, elem1}
	}))
}