package hrf

import (
	"github.com/v2pro/plz/countlog/spi"
	"github.com/v2pro/plz/msgfmt"
)

type propFormatter struct {
	fmt msgfmt.Formatter
}

func (formatter *propFormatter) Format(space []byte, event *spi.Event) []byte {
	return formatter.fmt.Format(space, event.Properties)
}
