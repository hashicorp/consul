package test

import (
	"testing"
	. "github.com/v2pro/plz/countlog"
	. "github.com/v2pro/plz/test"
	. "github.com/v2pro/plz/test/must"
)

func Test(t *testing.T) {
	t.Run("1 != 2", Case(func(ctx *Context) {
		Assert(1 == 2)
	}))
}
