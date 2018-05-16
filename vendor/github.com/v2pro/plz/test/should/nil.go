package should

import (
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/test/testify/assert"
	"runtime"
)

func Nil(actual interface{}) {
	t := test.CurrentT()
	if assert.Nil(t, actual) {
		return
	}
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Error("check failed")
		return
	}
	t.Error(test.ExtractFailedLines(file, line))
}

func AssertNil(actual interface{}) {
	t := test.CurrentT()
	if assert.Nil(t, actual) {
		return
	}
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Error("check failed")
		return
	}
	t.Error(test.ExtractFailedLines(file, line))
}

func NotNil(actual interface{}) {
	t := test.CurrentT()
	if assert.NotNil(t, actual) {
		return
	}
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Error("check failed")
		return
	}
	t.Error(test.ExtractFailedLines(file, line))
}

func AssertNotNil(actual interface{}) {
	t := test.CurrentT()
	if assert.NotNil(t, actual) {
		return
	}
	test.Helper()
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Error("check failed")
		return
	}
	t.Error(test.ExtractFailedLines(file, line))
}