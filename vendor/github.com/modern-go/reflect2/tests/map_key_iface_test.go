package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
	"github.com/modern-go/test/must"
)

type intError int

func (err intError) Error() string {
	return ""
}

func Test_map_iface_key(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[error]int{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, intError(2), 4)
		valType.SetIndex(obj, intError(2), 9)
		valType.SetIndex(obj, nil, 9)
		must.Panic(func() {
			valType.SetIndex(obj, "", 9)
		})
		return obj
	}))
	t.Run("GetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[error]int{intError(3): 9, intError(2): 4}
		valType := api.TypeOf(obj).(reflect2.MapType)
		must.Panic(func() {
			valType.GetIndex(obj, "")
		})
		return []interface{}{
			valType.GetIndex(obj, intError(3)),
			valType.GetIndex(obj, nil),
		}
	}))
	t.Run("Iterate", testOp(func(api reflect2.API) interface{} {
		obj := map[error]int{intError(2): 4}
		valType := api.TypeOf(obj).(reflect2.MapType)
		iter := valType.Iterate(obj)
		must.Pass(iter.HasNext(), "api", api)
		key1, elem1 := iter.Next()
		must.Pass(!iter.HasNext(), "api", api)
		return []interface{}{key1, elem1}
	}))
}
