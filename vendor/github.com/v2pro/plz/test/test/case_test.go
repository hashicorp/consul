package test

import (
	"testing"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/test/should"
	"github.com/v2pro/plz/test/must"
	"os"
	"errors"
)

func Test_case(t *testing.T) {
	t.Run("should.Pass will not exit", test.Case(func(ctx *countlog.Context) {
		should.Pass(1 == 1)
		should.Pass(1 == 2)
		ctx.Info("hello")
	}))
	t.Run("must.Pass will exit", test.Case(func(ctx *countlog.Context) {
		must.Pass(1 == 1)
		must.Pass(1 == 2)
		ctx.Info("hello")
	}))
	t.Run("multiline", test.Case(func(ctx *countlog.Context) {
		var f = func(i int) int { return i }
		must.Pass(struct{ i int }{
			f(100),
		} == struct{ i int }{
			f(101),
		})
	}))
	t.Run("attach details to assert", test.Case(func(ctx *countlog.Context) {
		a := 1
		b := 2
		should.Pass(a > b, "a", a, "b", b)
	}))
	t.Run("equal", test.Case(func(ctx *countlog.Context) {
		map1 := map[string]string{
			"a": "b",
			"c": "hello",
		}
		map2 := map[string]string{
			"a": "b",
			"c": "hi",
		}
		should.Equal(map1, map2)
	}))
	t.Run("nil", test.Case(func(ctx *countlog.Context) {
		should.Nil(errors.New("hello"))
	}))
	t.Run("failed call", test.Case(func(ctx *countlog.Context) {
		should.Call(os.Stat, "/tmp/no such file")
	}))
	t.Run("successful call", test.Case(func(ctx *countlog.Context) {
		var stat os.FileInfo
		should.Call(os.Stat, "/bin/bash").Set(&stat)
		should.Equal("bash", stat.Name())
	}))
	t.Run("successful call 2", test.Case(func(ctx *countlog.Context) {
		ret := should.Call(os.Stat, "/bin/bash")
		should.Equal("bash", ret[0].(os.FileInfo).Name())
	}))
}
