package stats

import "github.com/v2pro/plz/countlog/spi"

type countEvent struct {
	*Window
	site *spi.LogSite
	extractor dimensionExtractor
}

func (state *countEvent) Handle(event *spi.Event) {
	lock, dimensions := state.Window.Mutate()
	lock.Lock()
	counter := state.extractor.Extract(event, dimensions, NewCounterMonoid)
	*(counter.(*CounterMonoid)) += CounterMonoid(1)
	lock.Unlock()
}

func (state *countEvent) GetWindow() *Window {
	return state.Window
}

func (state *countEvent) LogSite() *spi.LogSite {
	return state.site
}