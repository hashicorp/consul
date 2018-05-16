package spi

type FuncEventSink func(site *LogSite) EventHandler

func (sink FuncEventSink) HandlerOf(site *LogSite) EventHandler {
	return sink(site)
}

type FuncEventHandler func(event *Event)

func (handler FuncEventHandler) Handle(event *Event) {
	handler(event)
}

func (handler FuncEventHandler) LogSite() *LogSite {
	return nil
}
