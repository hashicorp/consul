package test

import (
	"testing"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/dump"
	"github.com/v2pro/plz/test/must"
)

func Test_level0(t *testing.T) {
	t.Run("int8", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
		"__root__": {
			"type": "int8",
			"data": {
				"__ptr__": "{ptr1}"
			}
		},
		"{ptr1}": 100}`, dump.Var{int8(100)}.String())
	}))
}
