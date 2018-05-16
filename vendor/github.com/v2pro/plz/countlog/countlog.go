package countlog

import (
	"unsafe"
	"runtime"
	"time"
	"github.com/v2pro/plz/countlog/spi"
	"github.com/v2pro/plz/msgfmt"
	"errors"
	"github.com/v2pro/plz/concurrent"
)

const LevelTraceCall = spi.LevelTraceCall
const LevelTrace = spi.LevelTrace
const LevelDebugCall = spi.LevelDebugCall
const LevelDebug = spi.LevelDebug
const LevelInfoCall = spi.LevelInfoCall
const LevelInfo = spi.LevelInfo
const LevelWarn = spi.LevelWarn
const LevelError = spi.LevelError
const LevelFatal = spi.LevelFatal

func init() {
	concurrent.LogInfo = Info
	concurrent.LogPanic = LogPanic
}

func SetMinLevel(level int) {
	spi.MinLevel = level
	spi.MinCallLevel = level + 5
}

func ShouldLog(level int) bool {
	return level >= spi.MinLevel
}

func Trace(event string, properties ...interface{}) {
	if LevelTrace < spi.MinLevel {
		return
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelTrace, event, "", nil, nil, castEmptyInterfaces(uintptr(ptr)))
}

// TraceCall will calculate stats in TRACE level
// TraceCall will output individual log entries in TRACE_CALL level
func TraceCall(event string, err error, properties ...interface{}) error {
	if err != nil {
		ptr := unsafe.Pointer(&properties)
		return log(LevelWarn, event, "call", nil, err, castEmptyInterfaces(uintptr(ptr)))
	}
	if LevelTrace < spi.MinLevel {
		return nil
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelTrace, event, "call", nil, err, castEmptyInterfaces(uintptr(ptr)))
	return nil
}

func Debug(event string, properties ...interface{}) {
	if LevelDebug < spi.MinLevel {
		return
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelDebug, event, "", nil, nil, castEmptyInterfaces(uintptr(ptr)))
}

// DebugCall will calculate stats in DEBUG level
// DebugCall will output individual log entries in DEBUG_CALL level (TRACE includes DEBUG_CALL)
func DebugCall(event string, err error, properties ...interface{}) error {
	if err != nil {
		ptr := unsafe.Pointer(&properties)
		return log(LevelWarn, event, "call", nil, err, castEmptyInterfaces(uintptr(ptr)))
	}
	if LevelDebug < spi.MinLevel {
		return nil
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelDebug, event, "call", nil, err, castEmptyInterfaces(uintptr(ptr)))
	return nil
}

func Info(event string, properties ...interface{}) {
	if LevelInfo < spi.MinLevel {
		return
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelInfo, event, "", nil, nil, castEmptyInterfaces(uintptr(ptr)))
}

// InfoCall will calculate stats in INFO level
// InfoCall will output individual log entries in INFO_CALL level (DEBUG includes INFO_CALL)
func InfoCall(event string, err error, properties ...interface{}) error {
	if err != nil {
		ptr := unsafe.Pointer(&properties)
		return log(LevelWarn, event, "call", nil, err, castEmptyInterfaces(uintptr(ptr)))
	}
	if LevelInfo < spi.MinLevel {
		return nil
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelInfo, event, "call", nil, err, castEmptyInterfaces(uintptr(ptr)))
	return nil
}

func Warn(event string, properties ...interface{}) {
	ptr := unsafe.Pointer(&properties)
	log(LevelWarn, event, "", nil, nil, castEmptyInterfaces(uintptr(ptr)))
}

func Error(event string, properties ...interface{}) {
	ptr := unsafe.Pointer(&properties)
	log(LevelError, event, "", nil, nil, castEmptyInterfaces(uintptr(ptr)))
}

func Fatal(event string, properties ...interface{}) {
	ptr := unsafe.Pointer(&properties)
	log(LevelFatal, event, "", nil, nil, castEmptyInterfaces(uintptr(ptr)))
}

func Log(level int, event string, properties ...interface{}) {
	if level < spi.MinLevel {
		return
	}
	ptr := unsafe.Pointer(&properties)
	log(level, event, "", nil, nil, castEmptyInterfaces(uintptr(ptr)))
}

func LogPanic(recovered interface{}, properties ...interface{}) interface{} {
	if recovered == nil {
		return nil
	}
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, false)
	if len(properties) > 0 {
		properties = append(properties, "err", recovered, "stacktrace", string(buf))
		Fatal("event!panic", properties...)
	} else {
		Fatal("event!panic", "err", recovered, "stacktrace", string(buf))
	}
	return recovered
}

var handlerCache = concurrent.NewMap()

func log(level int, eventName string, agg string, ctx *Context, err error, properties []interface{}) error {
	handler := getHandler(eventName, agg, ctx, properties)
	event := &spi.Event{
		Level:      level,
		Context:    ctx,
		Error:      err,
		Timestamp:  time.Now(),
		Properties: properties,
	}
	ptr := unsafe.Pointer(event)
	castedEvent := castEvent(uintptr(ptr))
	handler.Handle(castedEvent)
	if castedEvent.Error != nil {
		formatter := msgfmt.FormatterOf(eventName, properties)
		errMsg := formatter.Format(nil, properties)
		errMsg = append(errMsg, ": "...)
		errMsg = append(errMsg, castedEvent.Error.Error()...)
		castedEvent.Error = errors.New(string(errMsg))
	}
	return castedEvent.Error
}

func addMemo(level int, eventName string, agg string, ctx *Context, err error, properties []interface{}) {
	event := &spi.Event{
		Level:      level,
		Context:    ctx,
		Error:      err,
		Timestamp:  time.Now(),
		Properties: properties,
	}
	ptr := unsafe.Pointer(event)
	castedEvent := castEvent(uintptr(ptr))
	if castedEvent.Error != nil {
		formatter := msgfmt.FormatterOf(eventName, properties)
		errMsg := formatter.Format(nil, properties)
		errMsg = append(errMsg, ": "...)
		errMsg = append(errMsg, castedEvent.Error.Error()...)
		castedEvent.Error = errors.New(string(errMsg))
	}
}

func castEmptyInterfaces(ptr uintptr) []interface{} {
	return *(*[]interface{})(unsafe.Pointer(ptr))
}

func castEvent(ptr uintptr) *spi.Event {
	return (*spi.Event)(unsafe.Pointer(ptr))
}
func castString(ptr uintptr) string {
	return *(*string)(unsafe.Pointer(ptr))
}

func getHandler(event string, agg string, ctx *Context, properties []interface{}) spi.EventHandler {
	handler, found := handlerCache.Load(event)
	if found {
		return handler.(spi.EventHandler)
	}
	return newHandler(event, agg, ctx, properties)
}

func newHandler(eventName string, agg string, ctx *Context, properties []interface{}) spi.EventHandler {
	pc, callerFile, callerLine, _ := runtime.Caller(4)
	site := &spi.LogSite{
		Context: ctx,
		Func: runtime.FuncForPC(pc).Name(),
		Event:  eventName,
		Agg:    agg,
		File:   callerFile,
		Line:   callerLine,
		Sample: properties,
	}
	handler := newRootHandler(site, nomalModeOnPanic)
	handlerCache.Store(eventName, handler)
	return handler
}
