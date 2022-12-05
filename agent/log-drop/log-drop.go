package log_drop

import (
	"context"
	"github.com/armon/go-metrics"
)

type Level int

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
)

const logCHDepth = 100

//go:generate mockery --name Logger --inpackage
type Logger interface {
	Info(string, ...interface{})
}

type log struct {
	s string
	i []interface{}
	l Level
}

type logDrop struct {
	logger Logger
	logCH  chan log
	name   string
}

func (r *logDrop) Info(s string, i ...interface{}) {
	r.pushLog(log{l: INFO, s: s, i: i})
}

func (r *logDrop) pushLog(l log) {
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
			switch l.l {
			case INFO:
				r.logger.Info(l.s, l.i)
			}
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
