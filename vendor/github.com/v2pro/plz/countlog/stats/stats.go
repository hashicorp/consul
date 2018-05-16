package stats

import (
	"github.com/v2pro/plz/countlog/spi"
)

type EventAggregator struct {
	executor  Executor
	collector Collector
}

type EventAggregatorConfig struct {
	Executor  Executor
	Collector Collector
}

func NewEventAggregator(cfg EventAggregatorConfig) *EventAggregator {
	executor := cfg.Executor
	if executor == nil {
		executor = DefaultExecutor
	}
	return &EventAggregator{
		executor:  executor,
		collector: cfg.Collector,
	}
}

func (aggregator *EventAggregator) HandlerOf(site *spi.LogSite) spi.EventHandler {
	if site.Agg != "" {
		return aggregator.createHandler(site.Agg, site)
	}
	sample := site.Sample
	for i := 0; i < len(sample); i += 2 {
		if sample[i].(string) == "agg" {
			return aggregator.createHandler(sample[i+1].(string), site)
		}
	}
	return nil
}

func (aggregator *EventAggregator) createHandler(agg string, site *spi.LogSite) spi.EventHandler {
	if aggregator.collector == nil {
		// disable aggregation if collector not set
		return &spi.DummyEventHandler{Site: site}
	}
	extractor, dimensionElemCount := newDimensionExtractor(site)
	window := newWindow(aggregator.executor, aggregator.collector, dimensionElemCount)
	switch agg {
	case "counter":
		return &countEvent{
			Window:    window,
			extractor: extractor,
			site:      site,
		}
	default:
		// TODO: log unknown agg
	}
	return nil
}
