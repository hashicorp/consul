package countlog

import (
	"github.com/v2pro/plz/countlog/spi"
	"runtime"
	"fmt"
	"runtime/debug"
)

type panicHandler func(recovered interface{}, event *spi.Event, site *spi.LogSite)

func newRootHandler(site *spi.LogSite, onPanic panicHandler) spi.EventHandler {
	statsHandler := EventAggregator.HandlerOf(site)
	if statsHandler == nil {
		return &oneHandler{
			site:    site,
			handler: EventWriter.HandlerOf(site),
			onPanic: onPanic,
		}
	}
	return &statsAndOutput{
		site:          site,
		statsHandler:  statsHandler,
		outputHandler: EventWriter.HandlerOf(site),
		onPanic:       onPanic,
	}
}

type oneHandler struct {
	site    *spi.LogSite
	handler spi.EventHandler
	onPanic panicHandler
}

func (handler *oneHandler) Handle(event *spi.Event) {
	defer func() {
		recovered := recover()
		if recovered != nil {
			handler.onPanic(recovered, event, handler.site)
		}
	}()
	handler.handler.Handle(event)
}

func (handler *oneHandler) LogSite() *spi.LogSite {
	return handler.site
}

type statsAndOutput struct {
	site          *spi.LogSite
	statsHandler  spi.EventHandler
	outputHandler spi.EventHandler
	onPanic       panicHandler
}

func (handler *statsAndOutput) Handle(event *spi.Event) {
	defer func() {
		recovered := recover()
		if recovered != nil {
			handler.onPanic(recovered, event, handler.site)
		}
	}()
	if event.Level >= spi.MinCallLevel {
		handler.outputHandler.Handle(event)
	}
	handler.statsHandler.Handle(event)
}

func (handler *statsAndOutput) LogSite() *spi.LogSite {
	return handler.site
}

func nomalModeOnPanic(recovered interface{}, event *spi.Event, site *spi.LogSite) {
	redirector := &redirector{
		site:            *site,
	}
	handlerCache.Store(site.Event, redirector)
	newSite := *site
	newSite.File = "unknown"
	newSite.Line = 0
	newSite.Sample = event.Properties
	newRootHandler(&newSite, fallbackModeOnPanic).Handle(event)
}

func fallbackModeOnPanic(recovered interface{}, event *spi.Event, site *spi.LogSite) {
	spi.OnError(fmt.Errorf("%v", recovered))
	if spi.MinLevel <= spi.LevelDebug {
		debug.PrintStack()
	}
}

type redirector struct {
	site spi.LogSite
}

func (redirector *redirector) Handle(event *spi.Event) {
	_, callerFile, callerLine, _ := runtime.Caller(3)
	key := accurateHandlerKey{callerFile, callerLine}
	handlerObj, found := handlerCache.Load(key)
	if found {
		handlerObj.(spi.EventHandler).Handle(event)
		return
	}
	site := redirector.site
	site.File = callerFile
	site.Line = callerLine
	site.Sample = event.Properties
	handler := newRootHandler(&site, fallbackModeOnPanic)
	handlerCache.Store(key, handler)
	handler.Handle(event)
	return
}

type accurateHandlerKey struct {
	File string
	Line int
}
