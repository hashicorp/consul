package must

import (
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/test/testify/assert"
	"runtime"
)

func Panic(f func()) (recovered interface{}) {
	t := test.CurrentT()
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("check failed")
		return
	}
	defer func() {
		recovered = recover()
		if assert.NotNil(t, recovered) {
			return
		}
		t.Fatal(test.ExtractFailedLines(file, line))
	}()
	f()
	return nil
}
