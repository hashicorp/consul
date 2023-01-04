package logdrop

import (
	"context"
	"github.com/hashicorp/go-hclog"
)

// SinkAdapter mimic the interface from hclog.SinkAdapter
//
//go:generate mockery --name SinkAdapter --inpackage
type SinkAdapter interface {
	Accept(name string, level hclog.Level, msg string, args ...interface{})
}

type Log struct {
	n string
	s string
	i []interface{}
	l hclog.Level
}

type logDropSink struct {
	sink   SinkAdapter
	logCh  chan Log
	name   string
	dropFn func(l Log)
}

// Accept consume a log and push it into a channel,
// if the channel is filled it will call dropFn
func (r *logDropSink) Accept(name string, level hclog.Level, msg string, args ...interface{}) {
	r.pushLog(Log{n: name, l: level, s: msg, i: args})
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
			r.sink.Accept(l.n, l.l, l.s, l.i)
		case <-ctx.Done():
			return
		}
	}
}

// NewLogDropSink create a log SinkAdapter that wrap another SinkAdapter
// It also create a go routine for consuming logs, the given context need to be canceled
// to properly deallocate the SinkAdapter.
func NewLogDropSink(ctx context.Context, name string, depth int, sink SinkAdapter, dropFn func(l Log)) hclog.SinkAdapter {
	r := &logDropSink{
		sink:   sink,
		logCh:  make(chan Log, depth),
		name:   name,
		dropFn: dropFn,
	}
	go r.logConsumer(ctx)
	return r
}
