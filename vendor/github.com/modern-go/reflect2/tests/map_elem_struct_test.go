package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
	"time"
)

func Test_map_elem_struct(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[int]time.Time{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, time.Time{})
		valType.SetIndex(obj, 3, time.Time{})
		return obj
	}))
	t.Run("SetIndex single ptr struct", testOp(func(api reflect2.API) interface{} {
		type TestObject struct {
			Field1 *int
		}
		obj := map[int]TestObject{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, TestObject{})
		valType.SetIndex(obj, 3, TestObject{})
		return obj
	}))
	t.Run("SetIndex single map struct", testOp(func(api reflect2.API) interface{} {
		type TestObject struct {
			Field1 map[int]int
		}
		obj := map[int]TestObject{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, TestObject{})
		valType.SetIndex(obj, 3, TestObject{})
		return obj
	}))
	t.Run("SetIndex single chan struct", testOp(func(api reflect2.API) interface{} {
		type TestObject struct {
			Field1 chan int
		}
		obj := map[int]TestObject{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, TestObject{})
		valType.SetIndex(obj, 3, TestObject{})
		return obj
	}))
	t.Run("SetIndex single func struct", testOp(func(api reflect2.API) interface{} {
		type TestObject struct {
			Field1 func()
		}
		obj := map[int]TestObject{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, TestObject{})
		valType.SetIndex(obj, 3, TestObject{})
		return obj
	}))
}
