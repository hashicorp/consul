package test

import (
	"testing"
	"io"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
	"encoding/json"
	"github.com/v2pro/plz/reflect2"
)

func Test_map(t *testing.T) {
	t.Run("map int to int", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
			"1": 1
		}`, jsonfmt.MarshalToString(map[int]int{
			1: 1,
		}))
	}))
	t.Run("map string to int", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
			"hello": 1
		}`, jsonfmt.MarshalToString(map[string]int{
			"hello": 1,
		}))
	}))
	t.Run("map int to ptr int", test.Case(func(ctx *countlog.Context) {
		one := 1
		must.JsonEqual(`{
			"1": 1
		}`, jsonfmt.MarshalToString(map[int]*int{
			1: &one,
		}))
	}))
	t.Run("map eface to int", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
			"1": 1
		}`, jsonfmt.MarshalToString(map[interface{}]int{
			1: 1,
		}))
	}))
	t.Run("map int to eface", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
			"1": 1
		}`, jsonfmt.MarshalToString(map[int]interface{}{
			1: 1,
		}))
	}))
	t.Run("map int to iface", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
			"1": 1
		}`, jsonfmt.MarshalToString(map[int]io.Closer{
			1: TestCloser(1),
		}))
	}))
	t.Run("map string to eface", test.Case(func(ctx *countlog.Context) {
		must.JsonEqual(`{
			"hello": 1,
			"world": "yes"
		}`, jsonfmt.MarshalToString(map[string]interface{}{
			"hello": 1,
			"world": "yes",
		}))
	}))
}

func Benchmark_map_unsafe(b *testing.B) {
	encoder := jsonfmt.EncoderOf(reflect2.TypeOf(map[string]int{}))
	m := map[string]int{
		"hello": 1,
		"world": 3,
	}
	b.ReportAllocs()
	b.ResetTimer()
	space := []byte(nil)
	for i := 0; i < b.N; i++ {
		space = encoder.Encode(nil, space[:0], reflect2.PtrOf(m))
	}
}

func Benchmark_map_safe(b *testing.B) {
	m := map[string]int{
		"hello": 1,
		"world": 3,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(m)
	}
}
