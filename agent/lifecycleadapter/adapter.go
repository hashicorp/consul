// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package lifecycleadapter connects the standalone lifecycle module to
// Consul's logger and telemetry.
package lifecycleadapter

import (
	"sort"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-metrics/prometheus"

	"github.com/hashicorp/consul/lifecycle"
)

// Gauges are the gauges emitted by the lifecycle Exchange.
var Gauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"lifecycle", "phase"},
		Help: "Current phase of a lifecycle component: 1 starting, 2 ready, 3 degraded, 4 transitioning, 5 failed, 6 stopped, 0 removed.",
	},
	{
		Name: []string{"lifecycle", "state"},
		Help: "1 for the current state of a lifecycle component, 0 for states it has left.",
	},
}

// Counters are the counters emitted by the lifecycle Exchange.
var Counters = []prometheus.CounterDefinition{
	{
		Name: []string{"lifecycle", "transition"},
		Help: "Counts state transitions of a lifecycle component.",
	},
	{
		Name: []string{"lifecycle", "stuck"},
		Help: "Counts times a lifecycle component stayed in a state longer than its configured maximum.",
	},
	{
		Name: []string{"lifecycle", "drain_timeout"},
		Help: "Counts transitions that proceeded while in-flight leases were still held after the drain timeout.",
	},
	{
		Name: []string{"lifecycle", "invalid_transition"},
		Help: "Counts rejected lifecycle transitions. A non-zero value is a bug in the component.",
	},
	{
		Name: []string{"lifecycle", "observer_dropped"},
		Help: "Counts lifecycle notifications dropped because the observer queue was full.",
	},
}

type metricsAdapter struct{}

func (metricsAdapter) SetGauge(name []string, value float32, labels map[string]string) {
	metrics.SetGaugeWithLabels(name, value, toLabels(labels))
}

func (metricsAdapter) IncrCounter(name []string, value float32, labels map[string]string) {
	metrics.IncrCounterWithLabels(name, value, toLabels(labels))
}

func toLabels(in map[string]string) []metrics.Label {
	if len(in) == 0 {
		return nil
	}
	out := make([]metrics.Label, 0, len(in))
	for k, v := range in {
		out = append(out, metrics.Label{Name: k, Value: v})
	}
	// Map order is random; sink keys are built from label order, so sort to
	// keep one series per label set.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// NewExchange creates the process-wide lifecycle Exchange, reporting through
// the given logger and the global go-metrics sink. A nil logger disables the
// logging observer.
func NewExchange(logger hclog.Logger, opts ...lifecycle.Option) *lifecycle.Exchange {
	all := []lifecycle.Option{lifecycle.WithMetrics(metricsAdapter{})}
	if logger != nil {
		all = append(all, lifecycle.WithLogger(logger.Named("lifecycle")))
	}
	return lifecycle.NewExchange(append(all, opts...)...)
}
