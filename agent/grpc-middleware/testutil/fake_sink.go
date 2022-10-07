package testutil

import (
	"sync"

	"github.com/armon/go-metrics"
)

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
