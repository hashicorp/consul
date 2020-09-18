package telemetry

import (
	"time"

	"github.com/armon/go-metrics"
)

/// TODO(kit) Label stands in for labels
type Label struct {
	Key   string
	Value string
}

// PLSREVIEW(kit): I decided to directly wrap the API of go-metrics because it informs all of our call sites.
//  However, this does not need to be set in stone. For example, I renamed Incr to Inc. This is a good opportunity to
//  adjust the metrics API we use internally. Any suggestions for improvements would be welcome!
type MetricsClient interface {
	// A Gauge should retain the last value it is set to
	SetGauge(key []string, val float32, labels ...Label)

	// Counters accumulate
	IncrCounter(key []string, val float32, labels ...Label)

	// Convenience fn to capture durations for samples
	MeasureSince(key []string, start time.Time, labels ...Label)

	// TODO(kit): I have not thought about filters yet. If we don't update them outside of RuntimeCfg
	//  we don't need to offer an interface.

	// TODO(kit): Should we have functions for rendering out the contents of the Inmemsink or arbitrary metrics state?
	// 	That would allow us to use everything through a client, rather than having to keep a reference to and
	//  interact with the sink in our agent. However, that could be more of a go-metrics implementation detail than
	//  one that we can generalize. Maybe more research here is needed - or we just mirror go-metrics
	//  and refactor the interface down the line if needed?
	// Render() maybe Report()

	// todo(kit): temporary value passthroughs. We want to delete these when we migrate Consul to import lib/telemetry
	//  instead of go-metrics.
	GetClient() interface{}
	GetInmemSink() interface{}
}

// NoopClient does nothing. Is it pronounced like "boop" or "no op"? Up to you, friend.
type NoopClient struct{}

// Let the compiler check that we're implementing all of MetricsClient
var _ MetricsClient = &NoopClient{}

func (*NoopClient) SetGauge(key []string, val float32, labels ...Label)         {}
func (*NoopClient) IncrCounter(key []string, val float32, labels ...Label)      {}
func (*NoopClient) MeasureSince(key []string, start time.Time, labels ...Label) {}
func (*NoopClient) GetClient() interface{}                                      { return nil }
func (*NoopClient) GetInmemSink() interface{}                                   { return nil }

// Init wraps armonMetricsInit, returning its client, InmemSink, and any errors directly.
// todo(kit): As a follow-up to this PR, we want to migrate away from many packages in Consul importing go-metrics.Metrics.
//  Migrating is not possible while we use the InmemSink to serve requests. First I want to get the MetricsClient to a
//  place that we're happy with it. Then we can fix Init to return a MetricsClient and replace the go-metrics dependencies
//  throughout Consul.
func Init(cfg Config) (*metrics.Metrics, *metrics.InmemSink, error) {
	client, err := initArmonMetrics(cfg)
	/*
		// Example client usage. lib/telemetry implements go-metrics and provides a condensed API
		client.SetGauge([]string{"consul", "api", "http"}, 20)

		// With Labels
		client.SetGauge([]string{"consul", "api", "http"}, 20, Label{"foo", "bar"}, Label{"baz", "qux"})

		// Apply labels slice to variadic fn
		labels := []Label{{"foo", "bar"}, {"baz", "qux"}}
		client.SetGauge([]string{"consul", "api", "http"}, 20, labels...)
	*/

	// fixme(kit): Unwrap the go-metrics components we use throughout Consul so we can still compile and run.
	//  Once this PR is out of draft phase, we plan on updating all go-metrics deps within Consul to use lib/telemetry
	//  instead.
	m := client.GetClient().(*metrics.Metrics)
	s := client.GetInmemSink().(*metrics.InmemSink)
	return m, s, err
}
