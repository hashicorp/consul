package telemetry

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestGaugeStore(t *testing.T) {
	globalGauges := &gaugeStore{
		store: make(map[string]*gaugeValue, 0),
		mutex: sync.Mutex{},
	}

	attributes := []attribute.KeyValue{
		{
			Key:   attribute.Key("test_key"),
			Value: attribute.StringValue("test_value"),
		},
	}

	globalGauges.Store("test", float64(1.23), attributes)

	// Should store a new gauge.
	val, ok := globalGauges.LoadAndDelete("test")
	require.True(t, ok)
	require.Equal(t, val.Value, float64(1.23))
	require.Equal(t, val.Attributes, attributes)

	// Gauge with key "test" have been deleted.
	val, ok = globalGauges.LoadAndDelete("test")
	require.False(t, ok)
	require.Nil(t, val)

	globalGauges.Store("duplicate", float64(1.5), nil)
	globalGauges.Store("duplicate", float64(6.7), nil)

	// Gauge with key "duplicate" should hold the latest (last seen) value.
	val, ok = globalGauges.LoadAndDelete("duplicate")
	require.True(t, ok)
	require.Equal(t, val.Value, float64(6.7))
}
