package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestGaugeStore(t *testing.T) {
	t.Parallel()

	globalGauges := NewGaugeStore()

	attributes := []attribute.KeyValue{
		{
			Key:   attribute.Key("test_key"),
			Value: attribute.StringValue("test_value"),
		},
	}

	globalGauges.Store("test", 1.23, attributes)

	// Should store a new gauge.
	val, ok := globalGauges.LoadAndDelete("test")
	require.True(t, ok)
	require.Equal(t, val.Value, 1.23)
	require.Equal(t, val.Attributes, attributes)

	// Gauge with key "test" have been deleted.
	val, ok = globalGauges.LoadAndDelete("test")
	require.False(t, ok)
	require.Nil(t, val)

	globalGauges.Store("duplicate", 1.5, nil)
	globalGauges.Store("duplicate", 6.7, nil)

	// Gauge with key "duplicate" should hold the latest (last seen) value.
	val, ok = globalGauges.LoadAndDelete("duplicate")
	require.True(t, ok)
	require.Equal(t, val.Value, 6.7)
}

func TestGaugeCallback_Failure(t *testing.T) {
	t.Parallel()

	k := "consul.raft.apply"
	globalGauges := NewGaugeStore()
	globalGauges.Store(k, 1.23, nil)

	cb := globalGauges.gaugeCallback(k)
	ctx, cancel := context.WithCancel(context.Background())

	cancel()
	err := cb(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}
