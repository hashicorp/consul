package must

import (
	"github.com/v2pro/plz/test"
	"runtime"
	"github.com/v2pro/plz/test/go-spew/spew"
)

//go:noinline
func Assert(result bool, kv ...interface{}) {
	if result {
		return
	}
	t := test.CurrentT()
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("check failed")
		return
	}
	t.Fatal(test.ExtractFailedLines(file, line))
}

//go:noinline
func Pass(result bool, kv ...interface{}) {
	if result {
		return
	}
	t := test.CurrentT()
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("check failed")
		return
	}
	for i := 0; i < len(kv); i+=2 {
		key := kv[i].(string)
		t.Errorf("%s: %s", key, spew.Sdump(kv[i+1]))
	}
	t.Fatal(test.ExtractFailedLines(file, line))
}
