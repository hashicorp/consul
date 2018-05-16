package printf

import (
	"github.com/v2pro/plz/countlog/spi"
	"github.com/v2pro/plz/msgfmt"
	"github.com/v2pro/plz/countlog/output"
	"time"
)

type Format struct {
	Layout string
}

func (format *Format) FormatterOf(site *spi.LogSite) output.Formatter {
	return &formatter{
		event:    site.Event,
		function: site.Func,
		file:     site.File,
		line:     site.Line,
		logFmt: msgfmt.FormatterOf(format.Layout + "\n",
			[]interface{}{
				"message", []byte{},
				"timestamp", time.Time{},
				"level", "",
				"event", "",
				"func", "",
				"file", "",
				"line", 0,
			}),
		messageFmt: msgfmt.FormatterOf(site.Event, site.Sample),
	}
}

type formatter struct {
	event      string
	function   string
	file       string
	line       int
	logFmt     msgfmt.Formatter
	messageFmt msgfmt.Formatter
}

func (formatter *formatter) Format(space []byte, event *spi.Event) []byte {
	return formatter.logFmt.Format(space,
		[]interface{}{
			"message", formatter.messageFmt.Format(nil, event.Properties),
			"timestamp", event.Timestamp,
			"level", spi.LevelName(event.Level),
			"event", formatter.event,
			"func", formatter.function,
			"file", formatter.file,
			"line", formatter.line,
		})
}
