package logdrop

import (
	"context"
	"github.com/hashicorp/go-hclog"
)

//go:generate mockery --name Logger --inpackage
type Logger interface {
	Log(level hclog.Level, msg string, args ...interface{})
}

type Log struct {
	s string
	i []interface{}
	l hclog.Level
}

type logDropSink struct {
	logger Logger
	logCh  chan Log
	name   string
	dropFn func(l Log)
}

func (r *logDropSink) Accept(name string, level hclog.Level, msg string, args ...interface{}) {
	r.pushLog(Log{l: level, s: msg, i: args})
}

func (r *logDropSink) pushLog(l Log) {
	select {
	case r.logCh <- l:
	default:
		r.dropFn(l)
	}
}

func (r *logDropSink) logConsumer(ctx context.Context) {
	for {
		select {
		case l := <-r.logCh:
			r.logger.Log(l.l, l.s, l.i)
		case <-ctx.Done():
			return
		}
	}
}

func NewLogDropSink(ctx context.Context, name string, depth int, logger Logger, dropFn func(l Log)) hclog.SinkAdapter {
	r := &logDropSink{
		logger: logger,
		logCh:  make(chan Log, depth),
		name:   name,
		dropFn: dropFn,
	}
	go r.logConsumer(ctx)
	return r
}
