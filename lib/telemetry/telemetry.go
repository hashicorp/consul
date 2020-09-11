package telemetry

import (
	"time"

	"github.com/armon/go-metrics"
)

type Label struct {
	Key   string
	Value string
}

// PLSREVIEW(kit): I decided to directly wrap the API of go-metrics because it informs all of our call sites.
//  However, this does not need to be set in stone. For example, I renamed Incr to Inc. This is a good opportunity
type MetricsClient interface {
	// A Gauge should retain the last value it is set to
	InitGauge(key []string)
	SetGauge(key []string, val float32)
	SetGaugeWithLabels(key []string, val float32, labels []Label)

	// KV should emit a Key/Value pair for each call
	InitKV(key []string)
	EmitKV(key []string, val float32)

	// Counters accumulate
	InitCounter(key []string)
	IncCounter(key []string, val float32)
	IncCounterWithLabels(key []string, val float32, labels []Label)

	// Samples are for timing quantiles
	InitSample(key []string)
	AddSample(key []string, val float32)
	AddSampleWithLabels(key []string, val float32, labels []Label)
	// Convenience fns to capture durations for samples
	MeasureSince(key []string, start time.Time)
	MeasureSinceWithLabels(key []string, start time.Time, labels []Label)
	// TODO(kit): I have not thought about filters yet. Do we need to migrate them through?
}

// NoopClient does nothing. Is it pronounced like "boop" or "no op"? Up to you, friend.
type NoopClient struct{}

// Let the compiler check that we're implementing all of MetricsClient
var _ MetricsClient = &NoopClient{}

func (*NoopClient) InitGauge(key []string)                                               {}
func (*NoopClient) SetGauge(key []string, val float32)                                   {}
func (*NoopClient) SetGaugeWithLabels(key []string, val float32, labels []Label)         {}
func (*NoopClient) InitKV(key []string)                                                  {}
func (*NoopClient) EmitKV(key []string, val float32)                                     {}
func (*NoopClient) InitCounter(key []string)                                             {}
func (*NoopClient) IncCounter(key []string, val float32)                                 {}
func (*NoopClient) IncCounterWithLabels(key []string, val float32, labels []Label)       {}
func (*NoopClient) InitSample(key []string)                                              {}
func (*NoopClient) AddSample(key []string, val float32)                                  {}
func (*NoopClient) AddSampleWithLabels(key []string, val float32, labels []Label)        {}
func (*NoopClient) MeasureSince(key []string, start time.Time)                           {}
func (*NoopClient) MeasureSinceWithLabels(key []string, start time.Time, labels []Label) {}

// Init wraps armonMetricsInit, returning its client, InmemSink, and any errors directly.
//  In the future, we wish to migrate away from using go-metrics directly. Instead, we'd like to use an abstraction so
//  we may swap backend telemetry clients as needs suit.
func Init(cfg Config) (*metrics.Metrics, *metrics.InmemSink, error) {
	return armonMetricsInit(cfg)
}
