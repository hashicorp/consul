package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
)

func Test_map_elem_array(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := map[int][2]*int{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, [2]*int{(*int)(reflect2.PtrOf(1)), (*int)(reflect2.PtrOf(2))})
		valType.SetIndex(obj, 3, [2]*int{(*int)(reflect2.PtrOf(3)), (*int)(reflect2.PtrOf(4))})
		return obj
	}))
	t.Run("SetIndex zero length array", testOp(func(api reflect2.API) interface{} {
		obj := map[int][0]*int{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, [0]*int{})
		valType.SetIndex(obj, 3, [0]*int{})
		return obj
	}))
	t.Run("SetIndex single ptr array", testOp(func(api reflect2.API) interface{} {
		obj := map[int][1]*int{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, [1]*int{(*int)(reflect2.PtrOf(1))})
		valType.SetIndex(obj, 3, [1]*int{})
		return obj
	}))
	t.Run("SetIndex single chan array", testOp(func(api reflect2.API) interface{} {
		obj := map[int][1]chan int{}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, [1]chan int{})
		valType.SetIndex(obj, 3, [1]chan int{})
		return obj
	}))
	t.Run("SetIndex single func array", testOp(func(api reflect2.API) interface{} {
		obj := map[int][1]func(){}
		valType := api.TypeOf(obj).(reflect2.MapType)
		valType.SetIndex(obj, 2, [1]func(){})
		valType.SetIndex(obj, 3, [1]func(){})
		return obj
	}))
}
