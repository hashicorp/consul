package spi

import "context"

var LogContextKey = 1010010001

type LogContext struct {
	Memos      [][]byte
	Properties []interface{}
}

func AddLogContext(ctx context.Context, key string, value interface{}) {
	logContext, _ := ctx.Value(LogContextKey).(*LogContext)
	if logContext == nil {
		return
	}
	logContext.Properties = append(logContext.Properties, key)
	logContext.Properties = append(logContext.Properties, value)
}

func GetLogContext(ctx context.Context) *LogContext {
	logContext, _ := ctx.Value(LogContextKey).(*LogContext)
	return logContext
}
