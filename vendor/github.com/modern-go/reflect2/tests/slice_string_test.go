package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
)

func Test_slice_string(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := []string{"hello", "world"}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.SetIndex(&obj, 0, "hi")
		valType.SetIndex(&obj, 1, "there")
		return obj
	}))
}