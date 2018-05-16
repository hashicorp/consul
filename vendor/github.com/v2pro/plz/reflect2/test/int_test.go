package test

import (
	"testing"
	"github.com/v2pro/plz/reflect2"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"unsafe"
	"github.com/v2pro/plz/test/must"
)

func Test_int(t *testing.T) {
	t.Run("New", testOp(func(api reflect2.API) interface{} {
		valType := api.TypeOf(1)
		obj := valType.New()
		*obj.(*int) = 100
		return obj
	}))
	t.Run("PackEFace", test.Case(func(ctx *countlog.Context) {
		valType := reflect2.TypeOf(1)
		hundred := 100
		must.Equal(&hundred, valType.PackEFace(unsafe.Pointer(&hundred)))
	}))
	t.Run("Indirect", test.Case(func(ctx *countlog.Context) {
		valType := reflect2.TypeOf(1)
		hundred := 100
		must.Equal(100, valType.Indirect(&hundred))
	}))
	t.Run("Indirect", test.Case(func(ctx *countlog.Context) {
		valType := reflect2.TypeOf(1)
		hundred := 100
		must.Equal(100, valType.UnsafeIndirect(unsafe.Pointer(&hundred)))
	}))
	t.Run("Set", testOp(func(api reflect2.API) interface{} {
		valType := api.TypeOf(1)
		i := 1
		j := 10
		valType.Set(&i, &j)
		return i
	}))
}
