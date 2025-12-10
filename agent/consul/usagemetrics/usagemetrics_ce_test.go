// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package usagemetrics

import (
	"testing"
	"time"

	"github.com/hashicorp/go-metrics"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/sdk/testutil"
)

func newStateStore() (*state.Store, error) {
	return state.NewStateStore(nil), nil
}

func TestUsageReporter_CE(t *testing.T) {
	getMetricsReporter := func(tc testCase) (*UsageMetricsReporter, *metrics.InmemSink, error) {
		// Only have a single interval for the test
		sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
		cfg := metrics.DefaultConfig("consul.usage.test")
		cfg.EnableHostname = false
		metrics.NewGlobal(cfg, sink)

		mockStateProvider := &mockStateProvider{}
		s, err := newStateStore()
		require.NoError(t, err)
		if tc.modifyStateStore != nil {
			tc.modifyStateStore(t, s)
		}
		mockStateProvider.On("State").Return(s)

		reporter, err := NewUsageMetricsReporter(
			new(Config).
				WithStateProvider(mockStateProvider).
				WithLogger(testutil.Logger(t)).
				WithDatacenter("dc1").
				WithGetMembersFunc(tc.getMembersFunc),
		)

		return reporter, sink, err
	}

	testUsageReporter_Tenantless(t, getMetricsReporter)
}
