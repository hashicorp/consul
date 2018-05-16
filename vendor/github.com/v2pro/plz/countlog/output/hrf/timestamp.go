package hrf

import (
	"github.com/v2pro/plz/countlog/spi"
)

type timestampFormatter struct {
}

func (formatter *timestampFormatter) Format(space []byte, event *spi.Event) []byte {
	return event.Timestamp.AppendFormat(space, "15:04:05.000")
}