package countlog

import (
	"context"
	"github.com/v2pro/plz/countlog/spi"
	"unsafe"
)

func Ctx(ctx context.Context) *Context {
	wrapped, isWrapped := ctx.(*Context)
	if isWrapped {
		return wrapped
	}
	return &Context{Context: ctx, logContext: &spi.LogContext{}}
}

type Context struct {
	context.Context
	logContext *spi.LogContext
	suppressedMinLevel int
}

func (ctx *Context) SuppressLevelsBelow(level int) {
	ctx.suppressedMinLevel = level
}

func (ctx *Context) Value(key interface{}) interface{} {
	if ctx == nil {
		return nil
	}
	if key == spi.LogContextKey {
		return ctx.logContext
	}
	return ctx.Context.Value(key)
}

func (ctx *Context) Trace(event string, properties ...interface{}) {
	if LevelTrace < spi.MinLevel {
		return
	}
	ptr := unsafe.Pointer(&properties)
	if LevelTrace < ctx.suppressedMinLevel {
		addMemo(LevelTrace, event, "", ctx, nil, castEmptyInterfaces(uintptr(ptr)))
		return
	}
	log(LevelTrace, event, "", ctx, nil, castEmptyInterfaces(uintptr(ptr)))
}

func (ctx *Context) TraceCall(event string, err error, properties ...interface{}) error {
	if err != nil {
		ptr := unsafe.Pointer(&properties)
		return log(LevelWarn, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	}
	if LevelTrace < spi.MinLevel {
		return nil
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelTrace, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	return nil
}

func (ctx *Context) Debug(event string, properties ...interface{}) {
	if LevelDebug < spi.MinLevel {
		return
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelDebug, event, "", ctx, nil, castEmptyInterfaces(uintptr(ptr)))
}

func (ctx *Context) DebugCall(event string, err error, properties ...interface{}) error {
	if err != nil {
		ptr := unsafe.Pointer(&properties)
		return log(LevelWarn, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	}
	if LevelDebug < spi.MinLevel {
		return nil
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelDebug, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	return nil
}

func (ctx *Context) Info(event string, properties ...interface{}) {
	if LevelInfo < spi.MinLevel {
		return
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelInfo, event, "", ctx, nil, castEmptyInterfaces(uintptr(ptr)))
}

func (ctx *Context) InfoCall(event string, err error, properties ...interface{}) error {
	if err != nil {
		ptr := unsafe.Pointer(&properties)
		return log(LevelWarn, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	}
	if LevelInfo < spi.MinLevel {
		return nil
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelInfo, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	return nil
}

func (ctx *Context) LogAccess(event string, err error, properties ...interface{}) error {
	if err != nil {
		ptr := unsafe.Pointer(&properties)
		return log(LevelError, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	}
	if LevelInfo < spi.MinLevel {
		return nil
	}
	ptr := unsafe.Pointer(&properties)
	log(LevelInfo, event, "call", ctx, err, castEmptyInterfaces(uintptr(ptr)))
	return nil
}

func (ctx *Context) Warn(event string, properties ...interface{}) {
	ptr := unsafe.Pointer(&properties)
	log(LevelWarn, event, "", ctx, nil, castEmptyInterfaces(uintptr(ptr)))
}

func (ctx *Context) Error(event string, properties ...interface{}) {
	ptr := unsafe.Pointer(&properties)
	log(LevelError, event, "", ctx, nil, castEmptyInterfaces(uintptr(ptr)))
}

func (ctx *Context) Fatal(event string, properties ...interface{}) {
	ptr := unsafe.Pointer(&properties)
	log(LevelFatal, event, "", ctx, nil, castEmptyInterfaces(uintptr(ptr)))
}

func (ctx *Context) Add(key string, value interface{}) {
	ctx.logContext.Properties = append(ctx.logContext.Properties, key)
	ctx.logContext.Properties = append(ctx.logContext.Properties, value)
}
