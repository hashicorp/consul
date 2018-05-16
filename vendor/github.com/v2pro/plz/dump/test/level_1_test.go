package test

import (
	"testing"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
	"github.com/v2pro/plz/dump"
)

func Test_level1(t *testing.T) {
	t.Run("string", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
		"__root__": {
			"type": "string",
			"data": {
				"__ptr__": "{ptr1}"
			}
		},
		"{ptr1}": {
			"data": {
				"__ptr__": "{ptr2}"
			},
			"len": 5
		},
		"{ptr2}": "hello"}`, dump.Var{"hello"}.String())
	}))
	t.Run("struct of pointer", test.Case(func(ctx *countlog.Context) {
		type TestObject struct {
			Field1 *int
			Field2 *int
		}
		one := 1
		two := 2
		obj := TestObject{&one, &two}
		must.JsonEqual(`{
		"__root__":{
			"type":"dump_test.TestObject",
			"data":{"__ptr__":"{ptr1}"}
		},
		"{ptr1}":{
			"Field1":{"__ptr__":"{ptr2}"},
			"Field2":{"__ptr__":"{ptr3}"}
		},
		"{ptr2}":1,
		"{ptr3}":2}`, dump.Var{Object: obj}.String())
	}))
}