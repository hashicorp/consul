package main

import (
	"testing"
	"github.com/v2pro/plz/countlog"
)

func Benchmark_trace_call(b *testing.B) {
	for i := 0; i < b.N; i++ {
		countlog.TraceCall("callee!doSomething", nil, "expensive", "hello")
	}
}

func Benchmark_expand(b *testing.B) {
	countlog.SetMinLevel(countlog.LevelInfo)
	for i := 0; i < b.N; i++ {
		countlog.Debug("event!some event", "expensive", func() interface{} {
			return make([]byte, 1024*1024*1024)
		})
	}
}

func Benchmark_no_expand(b *testing.B) {
	countlog.SetMinLevel(countlog.LevelInfo)
	for i := 0; i < b.N; i++ {
		countlog.Debug("event!some event", "expensive", "hello")
	}
}

var v1 = "v1"
var v2 = "v2"
var v3 = "v3"
var v4 = "v4"
var v5 = "v5"

func Benchmark_fixed_arg(b *testing.B) {
	countlog.MinLevel = countlog.LevelInfo
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		//if countlog.ShouldLog(countlog.LevelTrace) {
		//
		//}
		countlog.TraceCall5("callee!someThing", nil,
			"k1", v1,
			"k2", v2,
			"k3", v3,
			"k4", v4,
			"k5", v5)
	}
}
