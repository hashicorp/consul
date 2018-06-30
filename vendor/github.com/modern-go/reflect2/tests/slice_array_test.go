package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
)

func Test_slice_array(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := [][1]int{{}, {}}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.SetIndex(obj, 0, [1]int{1})
		valType.SetIndex(obj, 1, [1]int{2})
		return obj
	}))
	t.Run("SetIndex single ptr struct", testOp(func(api reflect2.API) interface{} {
		obj := [][1]*int{{}, {}}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.SetIndex(obj, 0, [1]*int{})
		valType.SetIndex(obj, 1, [1]*int{})
		return obj
	}))
	t.Run("SetIndex single chan struct", testOp(func(api reflect2.API) interface{} {
		obj := [][1]chan int{{}, {}}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.SetIndex(obj, 0, [1]chan int{})
		valType.SetIndex(obj, 1, [1]chan int{})
		return obj
	}))
	t.Run("SetIndex single func struct", testOp(func(api reflect2.API) interface{} {
		obj := [][1]func(){{}, {}}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.SetIndex(obj, 0, [1]func(){})
		valType.SetIndex(obj, 1, [1]func(){})
		return obj
	}))
}
