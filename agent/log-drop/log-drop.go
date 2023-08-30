// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package logdrop

import (
	"context"

	"github.com/hashicorp/go-hclog"
)

// Logger mimic the interface from hclog.Logger
//
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
	dropFn func(l Log)
}

// Accept consume a log and push it into a channel,
// if the channel is filled it will call dropFn
func (r *logDropSink) Accept(_ string, level hclog.Level, msg string, args ...interface{}) {
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
			r.logger.Log(l.l, l.s, l.i...)
		case <-ctx.Done():
			return
		}
	}
}

// NewLogDropSink create a log Logger that wrap another Logger
// It also create a go routine for consuming logs, the given context need to be canceled
// to properly deallocate the Logger.
func NewLogDropSink(ctx context.Context, depth int, logger Logger, dropFn func(l Log)) hclog.SinkAdapter {
	r := &logDropSink{
		logger: logger,
		logCh:  make(chan Log, depth),
		dropFn: dropFn,
	}
	go r.logConsumer(ctx)
	return r
}
