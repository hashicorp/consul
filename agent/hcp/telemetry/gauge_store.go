package telemetry

import (
	"sync"

	"go.opentelemetry.io/otel/attribute"
)

// Global store for Gauge values as workaround for async OpenTelemetry Gauge instrument.
var once sync.Once
var globalGauges *gaugeStore

type gaugeStore struct {
	store map[string]*gaugeValue
	mutex sync.Mutex
}

// gaugeValues hold both the float64 value and the labels.
type gaugeValue struct {
	Value      float64
	Attributes []attribute.KeyValue
}

// initGaugeStore initializes the global gauge store.
// initGaugeStore not thread-safe so it must only be init once.
func initGaugeStore() {
	// Avoid double initialization with sync.Once
	once.Do(func() {
		if globalGauges != nil {
			return
		}

		globalGauges = &gaugeStore{
			store: make(map[string]*gaugeValue, 0),
			mutex: sync.Mutex{},
		}
	})
}

// LoadAndDelete will read a Gauge value and delete it.
// Within the OTEL Gauge callbacks we must delete the value once we have read it
// to ensure we only emit a Gauge value once, as the callbacks continue to execute every collection cycle.
// The store must be initialized before using this method.
func (g *gaugeStore) LoadAndDelete(key string) (*gaugeValue, bool) {
	if g == nil {
		return nil, false
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()

	gauge, ok := g.store[key]

	delete(g.store, key)

	return gauge, ok
}

// Store adds a gaugeValue to the global gauge store.
// The store must be initialized before using this method.
func (g *gaugeStore) Store(key string, value float64, labels []attribute.KeyValue) {
	if g == nil {
		return
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()

	gv := &gaugeValue{
		Value:      value,
		Attributes: labels,
	}

	g.store[key] = gv
}
