package main

import (
	"github.com/v2pro/plz/countlog"
	"errors"
)

// when using --tags release, countlog.Trace will be empty and optimized away
//go:noinline
func trace_should_be_optimized_away() {
	countlog.Trace("event!trace can be optimized", "key", "value")
}

// if err != nil will not be checked twice when inlined
//go:noinline
func trace_call_should_combine_the_error_checking() int {
	err := doSomething()
	countlog.TraceCall("callee!doSomething", err)
	if err != nil {
		return 1
	}
	return 0
}

func doSomething() error {
	return errors.New("abc")
}

func main() {
	countlog.SetMinLevel(countlog.LevelTrace)
	trace_should_be_optimized_away()
	trace_call_should_combine_the_error_checking()
}
