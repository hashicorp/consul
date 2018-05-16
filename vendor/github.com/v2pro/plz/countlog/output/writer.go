package output

import (
	"github.com/v2pro/plz/countlog/spi"
	"io"
	"sync"
	"os"
)

type EventWriter struct {
	format Format
	writer io.Writer
}

type EventWriterConfig struct {
	Format Format
	Writer io.Writer
}

func NewEventWriter(cfg EventWriterConfig) *EventWriter {
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	writer = &recylceWriter{writer}
	return &EventWriter{
		format: cfg.Format,
		writer: writer,
	}
}

func (sink *EventWriter) HandlerOf(site *spi.LogSite) spi.EventHandler {
	formatter := sink.format.FormatterOf(site)
	return &writeEvent{
		site: site,
		formatter: formatter,
		writer:    sink.writer,
	}
}

type writeEvent struct {
	site *spi.LogSite
	formatter Formatter
	writer    io.Writer
}

func (handler *writeEvent) Handle(event *spi.Event) {
	space := bufPool.Get().([]byte)[:0]
	formatted := handler.formatter.Format(space, event)
	_, err := handler.writer.Write(formatted)
	if err != nil {
		spi.OnError(err)
	}
}

func (handler *writeEvent) LogSite() *spi.LogSite {
	return handler.site
}

var bufPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 128)
	},
}

type recylceWriter struct {
	writer io.Writer
}

func (writer *recylceWriter) Write(buf []byte) (int, error) {
	n, err := writer.writer.Write(buf)
	bufPool.Put(buf)
	return n, err
}
