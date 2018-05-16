package compact

import (
	"testing"
	"github.com/stretchr/testify/require"
	"time"
	"github.com/v2pro/plz/countlog/spi"
)

func Test_compact_string(t *testing.T) {
	should := require.New(t)
	now := time.Now()
	formatted := format(0, "event!abc", "file", 17, &spi.Event{
		Timestamp: now,
		Properties: []interface{}{
			"k1", "hello",
			"k2", []byte("abc"),
		},
	})
	should.Equal(`abc||timestamp=`+
		now.Format(time.RFC3339)+
		`||k1=hello||k2=abc`+ "\n", string(formatted))
}

func Test_callee(t *testing.T) {
	should := require.New(t)
	now := time.Now()
	formatted := format(0, "callee!abc", "file", 17, &spi.Event{
		Timestamp: now,
		Properties: []interface{}{
		},
	})
	should.Equal(`call abc||timestamp=`+now.Format(time.RFC3339)+"\n",
		string(formatted))
}

func Test_format_msg(t *testing.T) {
	should := require.New(t)
	now := time.Now()
	formatted := format(0, "{k1}~{k2}", "file", 17, &spi.Event{
		Timestamp: now,
		Properties: []interface{}{
			"k1", "hello",
			"k2", []byte("abc"),
		},
	})
	should.Equal(`hello~abc||timestamp=`+
		now.Format(time.RFC3339)+
		`||k1=hello||k2=abc`+ "\n", string(formatted))
}

func format(level int, eventName string,
	callerFile string, callerLine int, event *spi.Event) []byte {
	format := &Format{}
	formatter := format.FormatterOf(&spi.LogSite{
		File:   callerFile,
		Line:   callerLine,
		Event:  eventName,
		Sample: event.Properties,
	})
	return formatter.Format(nil, event)
}

func Benchmark_compact_string(b *testing.B) {
	format := &Format{}
	formatter := format.FormatterOf(&spi.LogSite{
		File:  "file",
		Line:  17,
		Event: "event!abc",
		Sample: []interface{}{
			"k1", "v1",
			"k2", []byte(nil),
		},
	})
	event := &spi.Event{
		Properties: []interface{}{
			"k1", "hello",
			"k2", []byte("中文"),
		},
	}
	var space []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		space = space[:0]
		space = formatter.Format(space, event)
	}
}
