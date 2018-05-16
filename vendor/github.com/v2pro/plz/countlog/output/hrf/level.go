package hrf

import (
	"github.com/v2pro/plz/countlog/spi"
)

type levelFormatter struct {
}

func (formatter *levelFormatter) Format(space []byte, event *spi.Event) []byte {
	return append(space, spi.ColoredLevelName(event.Level)...)
}
