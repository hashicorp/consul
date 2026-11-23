// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: BUSL-1.1

package lifecycleadapter

import (
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/lifecycle"
)

func TestNewExchange_EmitsMetrics(t *testing.T) {
	sink := metrics.NewInmemSink(10*time.Second, time.Minute)
	cfg := metrics.DefaultConfig("consul")
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false
	prev, err := metrics.NewGlobal(cfg, sink)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = metrics.NewGlobal(metrics.DefaultConfig("consul"), &metrics.BlackholeSink{}); _ = prev })

	ex := NewExchange(hclog.NewNullLogger())
	t.Cleanup(ex.Close)

	h, err := ex.Register(lifecycle.Restorable("server/state-store"))
	require.NoError(t, err)
	require.NoError(t, h.Transition(lifecycle.StateServing))

	require.Eventually(t, func() bool {
		for _, iv := range sink.Data() {
			iv.RLock()
			_, counter := iv.Counters["consul.lifecycle.transition;component=server/state-store;from=Booting;to=Serving"]
			iv.RUnlock()
			if counter {
				return true
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond)
}

func TestDefinitionsCoverEmittedMetrics(t *testing.T) {
	names := map[string]bool{}
	for _, d := range Gauges {
		names[d.Name[1]] = true
	}
	for _, d := range Counters {
		names[d.Name[1]] = true
	}
	for _, want := range []string{"phase", "state", "transition", "stuck", "drain_timeout", "invalid_transition", "observer_dropped"} {
		require.True(t, names[want], "missing definition for consul.lifecycle.%s", want)
	}
}
