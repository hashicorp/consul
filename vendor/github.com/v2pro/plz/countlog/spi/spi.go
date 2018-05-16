package spi

import (
	"context"
	"time"
	"fmt"
)

// MinLevel exists to minimize the overhead of Trace/Debug logging
var MinLevel = LevelTrace
// MinCallLevel will be half level higher than MinLevel
// to minimize the xxxCall output
var MinCallLevel = LevelDebugCall

// LevelTraceCall is lowest logging level
// enable this will print every TraceCall, which is a LOT
const LevelTraceCall = 5

// LevelTrace should be development environment default
const LevelTrace = 10

const LevelDebugCall = 15
const LevelDebug = 20
const LevelInfoCall = 25

// LevelInfo should be the production environment default
const LevelInfo = 30

// LevelWarn is the level for error != nil
const LevelWarn = 40

// LevelError is the level for user visible error
const LevelError = 50

// LevelFatal is the level for panic or panic like scenario
const LevelFatal = 60

func LevelName(level int) string {
	switch level {
	case LevelTraceCall, LevelTrace:
		return "TRACE"
	case LevelDebugCall, LevelDebug:
		return "DEBUG"
	case LevelInfoCall, LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogSite is the location of log in the source code
type LogSite struct {
	Context context.Context
	Func    string
	File    string
	Line    int
	Event   string
	Agg     string
	Sample  []interface{}
}

func (site *LogSite) LogContext() *LogContext {
	ctx := site.Context
	if ctx == nil {
		return nil
	}
	return GetLogContext(ctx)
}

func (site *LogSite) Location() string {
	return fmt.Sprintf("%s @ %s:%v", site.Func, site.File, site.Line)
}

type Event struct {
	Level      int
	Context    context.Context
	Error      error
	Timestamp  time.Time
	Properties []interface{}
}

type EventSink interface {
	HandlerOf(site *LogSite) EventHandler
}

type EventHandler interface {
	Handle(event *Event)
	LogSite() *LogSite
}

type EventHandlers []EventHandler

func (handlers EventHandlers) Handle(event *Event) {
	for _, handler := range handlers {
		handler.Handle(event)
	}
}

func (handlers EventHandlers) LogSite() *LogSite {
	return handlers[0].LogSite()
}

type DummyEventHandler struct {
	Site *LogSite
}

func (handler *DummyEventHandler) Handle(event *Event) {
}

func (handler *DummyEventHandler) LogSite() *LogSite {
	return handler.Site
}

type Memo struct {
	Site  *LogSite
	Event *Event
}
