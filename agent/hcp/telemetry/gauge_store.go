package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
)

type gaugeStore struct {
	store map[string]*gaugeValue
	mutex sync.Mutex
}

// gaugeValues hold both the float64 value and the labels.
type gaugeValue struct {
	Value      float64
	Attributes []attribute.KeyValue
}

// LoadAndDelete will read a Gauge value and delete it.
// Within the OTEL Gauge callbacks we must delete the value once we have read it
// to ensure we only emit a Gauge value once, as the callbacks continue to execute every collection cycle.
// The store must be initialized before using this method.
func (g *gaugeStore) LoadAndDelete(key string) (*gaugeValue, bool) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	gauge, ok := g.store[key]

	delete(g.store, key)

	return gauge, ok
}

// Store adds a gaugeValue to the global gauge store.
// The store must be initialized before using this method.
func (g *gaugeStore) Store(key string, value float64, labels []attribute.KeyValue) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	gv := &gaugeValue{
		Value:      value,
		Attributes: labels,
	}

	g.store[key] = gv
}

// gaugeCallback returns a callback which gets called when metrics are collected for export.
// the callback obtains the gauge value from the global gauges.
func (g *gaugeStore) gaugeCallback(key string) instrument.Float64Callback {
	// Closures keep a reference to the key string, that get garbage collected when code completes.
	return func(_ context.Context, obs instrument.Float64Observer) error {
		if gauge, ok := g.LoadAndDelete(key); ok {
			obs.Observe(gauge.Value, gauge.Attributes...)
		}
		return nil
	}
}
