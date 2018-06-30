package tests

import (
	"testing"
	"github.com/modern-go/reflect2"
	"github.com/modern-go/test"
	"unsafe"
	"github.com/modern-go/test/must"
	"context"
)

func Test_int(t *testing.T) {
	t.Run("New", testOp(func(api reflect2.API) interface{} {
		valType := api.TypeOf(1)
		obj := valType.New()
		*obj.(*int) = 100
		return obj
	}))
	t.Run("PackEFace", test.Case(func(ctx context.Context) {
		valType := reflect2.TypeOf(1)
		hundred := 100
		must.Equal(&hundred, valType.PackEFace(unsafe.Pointer(&hundred)))
	}))
	t.Run("Indirect", test.Case(func(ctx context.Context) {
		valType := reflect2.TypeOf(1)
		hundred := 100
		must.Equal(100, valType.Indirect(&hundred))
	}))
	t.Run("Indirect", test.Case(func(ctx context.Context) {
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
