package test

import (
	"testing"
	"github.com/v2pro/plz/countlog"
	"context"
	"github.com/v2pro/plz/gls"
	"reflect"
	"errors"
)

var testingTType = reflect.TypeOf((*testing.T)(nil))

func Case(testCase func(ctx *countlog.Context)) func(t *testing.T) {
	return func(t *testing.T) {
		goid := gls.GoID()
		gls.ResetGls(goid, map[interface{}]interface{}{
			testingTType: t,
		})
		ctx := countlog.Ctx(context.Background())
		defer func() {
			gls.DeleteGls(goid)
			if t.Failed() {
				ctx.LogAccess("test failed", errors.New(""))
			}
		}()
		testCase(ctx)
	}
}

func Skip(args ...interface{}) {
	CurrentT().Skip(args...)
}

func Skipf(format string, args ...interface{}) {
	CurrentT().Skipf(format, args...)
}

func CurrentT() *testing.T {
	t, found := gls.Get(testingTType).(*testing.T)
	if !found {
		panic("test not started with check.Case()")
	}
	return t
}
