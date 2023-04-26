// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"sync"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
)

func NewFakeSink(t *testing.T) (*FakeMetricsSink, *metrics.Metrics) {
	t.Helper()

	sink := &FakeMetricsSink{}
	cfg := &metrics.Config{
		ServiceName:      "testing",
		TimerGranularity: time.Millisecond, // Timers are in milliseconds
		ProfileInterval:  time.Second,      // Poll runtime every second
		FilterDefault:    true,
	}
	m, err := metrics.New(cfg, sink)
	require.NoError(t, err)
	return sink, m
}

type FakeMetricsSink struct {
	lock sync.Mutex
	metrics.BlackholeSink
	GaugeCalls       []MetricCall
	IncrCounterCalls []MetricCall
}

func (f *FakeMetricsSink) SetGaugeWithLabels(key []string, val float32, labels []metrics.Label) {
	f.lock.Lock()
	f.GaugeCalls = append(f.GaugeCalls, MetricCall{Key: key, Val: val, Labels: labels})
	f.lock.Unlock()
}

func (f *FakeMetricsSink) IncrCounterWithLabels(key []string, val float32, labels []metrics.Label) {
	f.lock.Lock()
	f.IncrCounterCalls = append(f.IncrCounterCalls, MetricCall{Key: key, Val: val, Labels: labels})
	f.lock.Unlock()
}

type MetricCall struct {
	Key    []string
	Val    float32
	Labels []metrics.Label
}
