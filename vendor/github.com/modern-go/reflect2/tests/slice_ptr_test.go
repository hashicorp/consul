package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
	"github.com/modern-go/test"

	"unsafe"
	"github.com/modern-go/test/must"
	"context"
)

func Test_slice_ptr(t *testing.T) {
	var pInt = func(val int) *int {
		return &val
	}
	t.Run("MakeSlice", testOp(func(api reflect2.API) interface{} {
		valType := api.TypeOf([]*int{}).(reflect2.SliceType)
		obj := valType.MakeSlice(5, 10)
		obj.([]*int)[0] = pInt(1)
		obj.([]*int)[4] = pInt(5)
		return obj
	}))
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := []*int{pInt(1), nil}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.SetIndex(obj, 0, pInt(2))
		valType.SetIndex(obj, 1, pInt(3))
		return obj
	}))
	t.Run("UnsafeSetIndex", test.Case(func(ctx context.Context) {
		obj := []*int{pInt(1), nil}
		valType := reflect2.TypeOf(obj).(reflect2.SliceType)
		valType.UnsafeSetIndex(reflect2.PtrOf(obj), 0, unsafe.Pointer(pInt(2)))
		valType.UnsafeSetIndex(reflect2.PtrOf(obj), 1, unsafe.Pointer(pInt(1)))
		must.Equal([]*int{pInt(2), pInt(1)}, obj)
	}))
	t.Run("GetIndex", testOp(func(api reflect2.API) interface{} {
		obj := []*int{pInt(1), nil}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		return []interface{}{
			valType.GetIndex(&obj, 0),
			valType.GetIndex(&obj, 1),
			valType.GetIndex(obj, 0),
			valType.GetIndex(obj, 1),
		}
	}))
	t.Run("Append", testOp(func(api reflect2.API) interface{} {
		obj := make([]*int, 2, 3)
		obj[0] = pInt(1)
		obj[1] = pInt(2)
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.Append(obj, pInt(3))
		// will trigger grow
		valType.Append(obj, pInt(4))
		return obj
	}))
}
