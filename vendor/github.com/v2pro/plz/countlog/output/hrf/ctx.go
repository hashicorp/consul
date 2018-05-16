package hrf

import (
	"github.com/v2pro/plz/msgfmt"
	"github.com/v2pro/plz/countlog/spi"
)

type ctxFormatter struct {
	fmt msgfmt.Formatter
}

func (formatter *ctxFormatter) Format(space []byte, event *spi.Event) []byte {
	return formatter.fmt.Format(space, spi.GetLogContext(event.Context).Properties)
}