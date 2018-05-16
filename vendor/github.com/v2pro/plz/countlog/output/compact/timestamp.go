package compact

import (
	"time"
	"github.com/v2pro/plz/countlog/spi"
)

type timestampFormatter struct {
}

func (formatter *timestampFormatter) Format(space []byte, event *spi.Event) []byte {
	space = append(space, "||timestamp="...)
	return event.Timestamp.AppendFormat(space, time.RFC3339)
}