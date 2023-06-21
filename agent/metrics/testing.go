// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package metrics

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
)

// Returns an in memory metrics sink for tests to assert metrics are emitted.
// Do not enable t.Parallel() since this relies on the global metrics instance.
func TestSetupMetrics(t *testing.T, serviceName string) *metrics.InmemSink {
	// Record for ages (5 mins) so we can be confident that our assertions won't
	// fail on silly long test runs due to dropped data.
	s := metrics.NewInmemSink(10*time.Second, 300*time.Second)
	cfg := metrics.DefaultConfig(serviceName)
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false
	metrics.NewGlobal(cfg, s)
	return s
}

// Asserts that a counter metric has the given value
func AssertCounter(t *testing.T, sink *metrics.InmemSink, name string, value float64) {
	t.Helper()

	data := sink.Data()

	var got float64
	for _, intv := range data {
		intv.RLock()
		// Note that InMemSink uses SampledValue and treats the _Sum_ not the Count
		// as the entire value.
		if sample, ok := intv.Counters[name]; ok {
			got += sample.Sum
		}
		intv.RUnlock()
	}

	if !assert.Equal(t, value, got) {
		// no nice way to dump this - this is copied from private method in
		// InMemSink used for dumping to stdout on SIGUSR1.
		buf := bytes.NewBuffer(nil)
		for _, intv := range data {
			intv.RLock()
			for name, val := range intv.Gauges {
				fmt.Fprintf(buf, "[%v][G] '%s': %0.3f\n", intv.Interval, name, val.Value)
			}
			for name, vals := range intv.Points {
				for _, val := range vals {
					fmt.Fprintf(buf, "[%v][P] '%s': %0.3f\n", intv.Interval, name, val)
				}
			}
			for name, agg := range intv.Counters {
				fmt.Fprintf(buf, "[%v][C] '%s': %s\n", intv.Interval, name, agg.AggregateSample)
			}
			for name, agg := range intv.Samples {
				fmt.Fprintf(buf, "[%v][S] '%s': %s\n", intv.Interval, name, agg.AggregateSample)
			}
			intv.RUnlock()
		}
		t.Log(buf.String())
	}
}

// Asserts that a gauge metric has the current value
func AssertGauge(t *testing.T, sink *metrics.InmemSink, name string, value float32) {
	t.Helper()

	data := sink.Data()

	// Loop backward through intervals until there is a non-empty one
	// Addresses flakiness around recording to one interval but accessing during the next
	var got float32
	for i := len(data) - 1; i >= 0; i-- {
		currentInterval := data[i]

		currentInterval.RLock()
		if len(currentInterval.Gauges) > 0 {
			got = currentInterval.Gauges[name].Value
			currentInterval.RUnlock()
			break
		}
		currentInterval.RUnlock()
	}

	if !assert.Equal(t, value, got) {
		buf := bytes.NewBuffer(nil)
		for _, intv := range data {
			intv.RLock()
			for name, val := range intv.Gauges {
				fmt.Fprintf(buf, "[%v][G] '%s': %0.3f\n", intv.Interval, name, val.Value)
			}
			intv.RUnlock()
		}
		t.Log(buf.String())
	}
}
