package test

import (
	"testing"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
)

func Test_bool(t *testing.T) {
	t.Run("true", test.Case(func(ctx *countlog.Context) {
		must.Equal("true", jsonfmt.MarshalToString(true))
	}))
}
