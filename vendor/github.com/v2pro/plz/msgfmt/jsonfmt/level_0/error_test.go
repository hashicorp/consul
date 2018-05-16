package test

import (
	"testing"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"errors"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
)

type testError struct {
	onePtr *int
}

func (err testError) Error() string {
	return "hello"
}

func Test_error(t *testing.T) {
	t.Run("ptr struct", test.Case(func(ctx *countlog.Context) {
		must.Equal(`"hello"`, jsonfmt.MarshalToString(errors.New("hello")))
	}))
	t.Run("single ptr struct", test.Case(func(ctx *countlog.Context) {
		type TestObject struct {
			Err testError
		}
		must.Equal(`{"Err":"hello"}`, jsonfmt.MarshalToString(TestObject{}))
	}))
}
