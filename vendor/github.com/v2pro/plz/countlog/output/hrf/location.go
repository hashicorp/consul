package hrf

import "github.com/v2pro/plz/countlog/spi"

type locationFormatter string

func (formatter locationFormatter) Format(space []byte, event *spi.Event) []byte {
	if event.Level >= spi.LevelWarn {
		return append(space, formatter...)
	}
	return space
}