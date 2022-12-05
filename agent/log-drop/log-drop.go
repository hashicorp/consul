package log_drop

import (
	"context"
	"github.com/armon/go-metrics"
)

const logCHDepth = 100

//go:generate mockery --name Logger --inpackage
type Logger interface {
	Info(string, ...interface{})
}

type log struct {
	s string
	i []interface{}
}

type logDrop struct {
	logger Logger
	logCH  chan log
	name   string
}

func (r *logDrop) Info(s string, i ...interface{}) {
	l := log{s: s, i: i}
	select {
	case r.logCH <- l:
	default:
		metrics.IncrCounter([]string{r.name, "log-dropped"}, 1)
	}
}

func (r *logDrop) logConsumer(ctx context.Context) {
	for {
		select {
		case l := <-r.logCH:
			r.logger.Info(l.s, l.i)
		case <-ctx.Done():
			return
		}
	}
}

func NewLogDrop(ctx context.Context, name string, logger Logger) Logger {
	r := &logDrop{
		logger: logger,
		logCH:  make(chan log, logCHDepth),
		name:   name,
	}
	go r.logConsumer(ctx)
	return r
}
