package output

import "github.com/v2pro/plz/countlog/spi"

type Format interface {
	FormatterOf(site *spi.LogSite) Formatter
}

type Formatter interface {
	Format(space []byte, event *spi.Event) []byte
}

type Formatters []Formatter

func (formatters Formatters) Format(space []byte, event *spi.Event) []byte {
	for _, formatter := range formatters {
		space = formatter.Format(space, event)
	}
	return space
}
