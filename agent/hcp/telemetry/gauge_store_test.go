package telemetry

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestGaugeStore(t *testing.T) {
	t.Parallel()

	gaugeStore := NewGaugeStore()

	attributes := []attribute.KeyValue{
		{
			Key:   attribute.Key("test_key"),
			Value: attribute.StringValue("test_value"),
		},
	}

	gaugeStore.Set("test", 1.23, attributes)

	// Should store a new gauge.
	val, ok := gaugeStore.LoadAndDelete("test")
	require.True(t, ok)
	require.Equal(t, val.Value, 1.23)
	require.Equal(t, val.Attributes, attributes)

	// Gauge with key "test" have been deleted.
	val, ok = gaugeStore.LoadAndDelete("test")
	require.False(t, ok)
	require.Nil(t, val)

	gaugeStore.Set("duplicate", 1.5, nil)
	gaugeStore.Set("duplicate", 6.7, nil)

	// Gauge with key "duplicate" should hold the latest (last seen) value.
	val, ok = gaugeStore.LoadAndDelete("duplicate")
	require.True(t, ok)
	require.Equal(t, val.Value, 6.7)
}

func TestGaugeCallback_Failure(t *testing.T) {
	t.Parallel()

	k := "consul.raft.apply"
	gaugeStore := NewGaugeStore()
	gaugeStore.Set(k, 1.23, nil)

	cb := gaugeStore.gaugeCallback(k)
	ctx, cancel := context.WithCancel(context.Background())

	cancel()
	err := cb(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}

// TestGaugeStore_Race induces a race condition. When run with go test -race,
// this test should pass if  implementation is concurrency safe.
func TestGaugeStore_Race(t *testing.T) {
	t.Parallel()

	gaugeStore := NewGaugeStore()
	wg := &sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		v := rand.Float64()
		uuid, err := uuid.GenerateUUID()
		require.NoError(t, err)

		go storeAndRetrieve(t, fmt.Sprintf("%s%f", uuid, v), v, gaugeStore, wg)
	}
	wg.Wait()
}

func storeAndRetrieve(t *testing.T, k string, v float64, gaugeStore *gaugeStore, wg *sync.WaitGroup) {
	gaugeStore.Set(k, v, nil)
	gv, ok := gaugeStore.LoadAndDelete(k)
	require.True(t, ok)
	require.Equal(t, v, gv.Value)
	wg.Done()
}
