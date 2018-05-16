package test

import (
	"testing"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/reflect2"
	"github.com/v2pro/plz/test/must"
	"github.com/v2pro/plz/test"
)

func Test_struct_ptr(t *testing.T) {
	type TestObject struct {
		Field1 *int
	}
	t.Run("PackEFace", test.Case(func(ctx *countlog.Context) {
		valType := reflect2.TypeOf(TestObject{})
		ptr := valType.UnsafeNew()
		must.Equal(&TestObject{}, valType.PackEFace(ptr))
	}))
	t.Run("Indirect", test.Case(func(ctx *countlog.Context) {
		valType := reflect2.TypeOf(TestObject{})
		must.Equal(TestObject{}, valType.Indirect(&TestObject{}))
	}))
}