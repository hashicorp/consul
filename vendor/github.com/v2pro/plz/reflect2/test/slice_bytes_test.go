package test

import (
	"testing"
	"github.com/v2pro/plz/reflect2"
)

func Test_slice_bytes(t *testing.T) {
	t.Run("SetIndex", testOp(func(api reflect2.API) interface{} {
		obj := [][]byte{[]byte("hello"), []byte("world")}
		valType := api.TypeOf(obj).(reflect2.SliceType)
		valType.SetIndex(&obj, 0, []byte("hi"))
		valType.SetIndex(&obj, 1, []byte("there"))
		return obj
	}))
}