// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package telemetry

import (
	"context"
	"fmt"
	"sync"
	"testing"

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
	samples := 100
	errCh := make(chan error, samples)
	for i := 0; i < samples; i++ {
		wg.Add(1)
		key := fmt.Sprintf("consul.test.%d", i)
		value := 12.34
		go func() {
			defer wg.Done()
			gaugeStore.Set(key, value, nil)
			gv, _ := gaugeStore.LoadAndDelete(key)
			if gv.Value != value {
				errCh <- fmt.Errorf("expected value: '%f', but got: '%f' for key: '%s'", value, gv.Value, key)
			}
		}()
	}

	wg.Wait()

	require.Empty(t, errCh)
}
