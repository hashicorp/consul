// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package usagemetrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
)

type mockStateProvider struct {
	mock.Mock
}

func (m *mockStateProvider) State() *state.Store {
	retValues := m.Called()
	return retValues.Get(0).(*state.Store)
}

func assertEqualGaugeMaps(t *testing.T, expectedMap, foundMap map[string]metrics.GaugeValue) {
	t.Helper()

	for key := range foundMap {
		if _, ok := expectedMap[key]; !ok {
			t.Errorf("found unexpected gauge key: %s with value: %v", key, foundMap[key])
		}
	}

	for key, expected := range expectedMap {
		if _, ok := foundMap[key]; !ok {
			t.Errorf("did not find expected gauge key: %s", key)
			continue
		}
		assert.Equal(t, expected, foundMap[key], "gauge key mismatch on %q", key)
	}
}

func BenchmarkRunOnce(b *testing.B) {
	const index = 123

	store := state.NewStateStore(nil)

	// This loop generates:
	//
	//	4 (service kind) * 100 (service) * 5 * (node) = 2000 proxy services. And 500 non-proxy services.
	for _, kind := range []structs.ServiceKind{
		// These will be included in the count.
		structs.ServiceKindConnectProxy,
		structs.ServiceKindIngressGateway,
		structs.ServiceKindTerminatingGateway,
		structs.ServiceKindMeshGateway,

		// This one will not.
		structs.ServiceKindTypical,
	} {
		for i := 0; i < 100; i++ {
			serviceName := fmt.Sprintf("%s-%d", kind, i)

			for j := 0; j < 5; j++ {
				nodeName := fmt.Sprintf("%s-node-%d", serviceName, j)

				require.NoError(b, store.EnsureRegistration(index, &structs.RegisterRequest{
					Node: nodeName,
					Service: &structs.NodeService{
						ID:      serviceName,
						Service: serviceName,
						Kind:    kind,
					},
				}))
			}
		}
	}

	benchmarkRunOnce(b, store)
}

func benchmarkRunOnce(b *testing.B, store *state.Store) {
	b.Helper()

	config := lib.TelemetryConfig{
		MetricsPrefix: "consul",
		FilterDefault: true,
		PrometheusOpts: prometheus.PrometheusOpts{
			Expiration: time.Second * 30,
			Name:       "consul",
		},
	}

	lib.InitTelemetry(config, hclog.NewNullLogger())

	um, err := NewUsageMetricsReporter(&Config{
		stateProvider:  benchStateProvider(func() *state.Store { return store }),
		logger:         hclog.NewNullLogger(),
		getMembersFunc: func() []serf.Member { return nil },
	})
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		um.runOnce()
	}
}

type benchStateProvider func() *state.Store

func (b benchStateProvider) State() *state.Store {
	return b()
}
