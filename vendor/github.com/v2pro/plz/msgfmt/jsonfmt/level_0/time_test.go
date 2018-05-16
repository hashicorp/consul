package test

import (
	"testing"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"time"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
)

func Test_time(t *testing.T) {
	t.Run("epoch", test.Case(func(ctx *countlog.Context) {
		must.Equal(`"0001-01-01T00:00:00Z"`, jsonfmt.MarshalToString(time.Time{}))
	}))
}
