package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// gaugeStore holds last seen Gauge values for a particular metric (<name,last_value>) in the store.
// OTEL does not currently have a synchronous Gauge instrument. Instead, it allows the registration of callbacks.
// The callbacks are called during export, where the Gauge value must be returned.
// This store is a workaround, which holds last seen Gauge values until the callback is called.
type gaugeStore struct {
	store map[string]*gaugeValue
	mutex sync.Mutex
}

// gaugeValues are the last seen measurement for a Gauge metric, which contains a float64 value and labels.
type gaugeValue struct {
	Value      float64
	Attributes []attribute.KeyValue
}

// LoadAndDelete will read a Gauge value and delete it.
// Once registered for a metric name, a Gauge callback will continue to execute every collection cycel.
// We must delete the value once we have read it, to avoid repeat values being sent.
func (g *gaugeStore) LoadAndDelete(key string) (*gaugeValue, bool) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	gauge, ok := g.store[key]
	if !ok {
		return nil, ok
	}

	delete(g.store, key)

	return gauge, ok
}

// Store adds a gaugeValue to the global gauge store.
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
func (g *gaugeStore) gaugeCallback(key string) metric.Float64Callback {
	// Closures keep a reference to the key string, that get garbage collected when code completes.
	return func(_ context.Context, obs metric.Float64Observer) error {
		if gauge, ok := g.LoadAndDelete(key); ok {
			obs.Observe(gauge.Value, metric.WithAttributes(gauge.Attributes...))
		}
		return nil
	}
}
