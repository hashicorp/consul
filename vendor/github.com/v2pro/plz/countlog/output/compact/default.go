package compact

import (
	"github.com/v2pro/plz/countlog/spi"
	"github.com/v2pro/plz/msgfmt"
)

type defaultFormatter struct {
	fmt msgfmt.Formatter
}

func (formatter *defaultFormatter) Format(space []byte, event *spi.Event) []byte {
	return formatter.fmt.Format(space, event.Properties)
}
