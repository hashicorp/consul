package test

import (
	"github.com/v2pro/plz/reflect2"
	"testing"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
	"github.com/v2pro/plz/test"
)

func testOp(f func(api reflect2.API) interface{}) func(t *testing.T) {
	return test.Case(func(ctx *countlog.Context) {
		unsafeResult := f(reflect2.ConfigUnsafe)
		safeResult := f(reflect2.ConfigSafe)
		must.Equal(safeResult, unsafeResult)
	})
}
