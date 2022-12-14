package logdrop

import (
	"context"
)

type Level int

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
)

//go:generate mockery --name Logger --inpackage
type Logger interface {
	Info(string, ...interface{})
}

type Log struct {
	s string
	i []interface{}
	l Level
}

type logDrop struct {
	logger Logger
	logCh  chan Log
	name   string
	dropFn func(l Log)
}

func (r *logDrop) Info(s string, i ...interface{}) {
	r.pushLog(Log{l: INFO, s: s, i: i})
}

func (r *logDrop) pushLog(l Log) {
	select {
	case r.logCh <- l:
	default:
		r.dropFn(l)
	}
}

func (r *logDrop) logConsumer(ctx context.Context) {
	for {
		select {
		case l := <-r.logCh:
			switch l.l {
			case INFO:
				r.logger.Info(l.s, l.i)
			}
		case <-ctx.Done():
			return
		}
	}
}

func NewLogDrop(ctx context.Context, name string, depth int, logger Logger, dropFn func(l Log)) Logger {
	r := &logDrop{
		logger: logger,
		logCh:  make(chan Log, depth),
		name:   name,
		dropFn: dropFn,
	}
	go r.logConsumer(ctx)
	return r
}
