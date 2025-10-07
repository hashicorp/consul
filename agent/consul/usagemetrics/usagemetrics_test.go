// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package usagemetrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-metrics/prometheus"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/version"
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

type testCase struct {
	modifyStateStore func(t *testing.T, s *state.Store)
	getMembersFunc   getMembersFunc
	expectedGauges   map[string]metrics.GaugeValue
}

var baseCases = map[string]testCase{
	"empty-state": {
		expectedGauges: map[string]metrics.GaugeValue{
			// --- node ---
			"consul.usage.test.state.nodes;datacenter=dc1": {
				Name:   "consul.usage.test.state.nodes",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- peering ---
			"consul.usage.test.state.peerings;datacenter=dc1": {
				Name:   "consul.usage.test.state.peerings",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- member ---
			"consul.usage.test.members.clients;datacenter=dc1": {
				Name:   "consul.usage.test.members.clients",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.members.servers;datacenter=dc1": {
				Name:   "consul.usage.test.members.servers",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service ---
			"consul.usage.test.state.services;datacenter=dc1": {
				Name:   "consul.usage.test.state.services",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.service_instances;datacenter=dc1": {
				Name:   "consul.usage.test.state.service_instances",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service mesh ---
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-proxy": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-proxy"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=mesh-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-native": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-native"},
				},
			},
			"consul.usage.test.state.billable_service_instances;datacenter=dc1": {
				Name:  "consul.usage.test.state.billable_service_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
				},
			},
			// --- kv ---
			"consul.usage.test.state.kv_entries;datacenter=dc1": {
				Name:   "consul.usage.test.state.kv_entries",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- config entries ---
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-intentions": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-intentions"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-resolver": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-resolver"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-router": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-router"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-splitter": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-splitter"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=mesh": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=proxy-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "proxy-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=exported-services": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "exported-services"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=sameness-group": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "sameness-group"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=bound-api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "bound-api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=file-system-certificate": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "file-system-certificate"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=inline-certificate": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "inline-certificate"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=http-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "http-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=tcp-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "tcp-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=jwt-provider": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "jwt-provider"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=control-plane-request-limit": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "control-plane-request-limit"},
				},
			},
			// --- version ---
			fmt.Sprintf("consul.usage.test.version;version=%s;pre_release=%s", versionWithMetadata(), version.VersionPrerelease): {
				Name:  "consul.usage.test.version",
				Value: 1,
				Labels: []metrics.Label{
					{Name: "version", Value: versionWithMetadata()},
					{Name: "pre_release", Value: version.VersionPrerelease},
				},
			},
		},
		getMembersFunc: func() []serf.Member { return []serf.Member{} },
	},
	"nodes": {
		modifyStateStore: func(t *testing.T, s *state.Store) {
			require.NoError(t, s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
			require.NoError(t, s.EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
		},
		getMembersFunc: func() []serf.Member {
			return []serf.Member{
				{
					Name:   "foo",
					Tags:   map[string]string{"role": "consul"},
					Status: serf.StatusAlive,
				},
				{
					Name:   "bar",
					Tags:   map[string]string{"role": "consul"},
					Status: serf.StatusAlive,
				},
			}
		},
		expectedGauges: map[string]metrics.GaugeValue{
			// --- node ---
			"consul.usage.test.state.nodes;datacenter=dc1": {
				Name:   "consul.usage.test.state.nodes",
				Value:  2,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- peering ---
			"consul.usage.test.state.peerings;datacenter=dc1": {
				Name:   "consul.usage.test.state.peerings",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- member ---
			"consul.usage.test.members.servers;datacenter=dc1": {
				Name:   "consul.usage.test.members.servers",
				Value:  2,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.members.clients;datacenter=dc1": {
				Name:   "consul.usage.test.members.clients",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service ---
			"consul.usage.test.state.services;datacenter=dc1": {
				Name:   "consul.usage.test.state.services",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.service_instances;datacenter=dc1": {
				Name:   "consul.usage.test.state.service_instances",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service mesh ---
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-proxy": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-proxy"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=mesh-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-native": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-native"},
				},
			},
			"consul.usage.test.state.billable_service_instances;datacenter=dc1": {
				Name:  "consul.usage.test.state.billable_service_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
				},
			},
			// --- kv ---
			"consul.usage.test.state.kv_entries;datacenter=dc1": {
				Name:   "consul.usage.test.state.kv_entries",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- config entries ---
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-intentions": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-intentions"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-resolver": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-resolver"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-router": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-router"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-splitter": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-splitter"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=mesh": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=proxy-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "proxy-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=exported-services": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "exported-services"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=sameness-group": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "sameness-group"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=bound-api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "bound-api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=file-system-certificate": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "file-system-certificate"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=inline-certificate": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "inline-certificate"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=http-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "http-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=tcp-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "tcp-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=jwt-provider": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "jwt-provider"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=control-plane-request-limit": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "control-plane-request-limit"},
				},
			},
			// --- version ---
			fmt.Sprintf("consul.usage.test.version;version=%s;pre_release=%s", versionWithMetadata(), version.VersionPrerelease): {
				Name:  "consul.usage.test.version",
				Value: 1,
				Labels: []metrics.Label{
					{Name: "version", Value: versionWithMetadata()},
					{Name: "pre_release", Value: version.VersionPrerelease},
				},
			},
		},
	},
}

func testUsageReporter_Tenantless(t *testing.T, getMetricsReporter func(tc testCase) (*UsageMetricsReporter, *metrics.InmemSink, error)) {
	t.Run("emitNodeUsage", func(t *testing.T) {
		testUsageReporter_emitNodeUsage_CE(t, getMetricsReporter)
	})

	t.Run("emitPeeringUsage", func(t *testing.T) {
		testUsageReporter_emitPeeringUsage_CE(t, getMetricsReporter)
	})

	t.Run("emitServiceUsage", func(t *testing.T) {
		testUsageReporter_emitServiceUsage_CE(t, getMetricsReporter)
	})

	t.Run("emitKVUsage", func(t *testing.T) {
		testUsageReporter_emitKVUsage_CE(t, getMetricsReporter)
	})
}

func testUsageReporter_emitNodeUsage_CE(t *testing.T, getReporter func(testCase) (*UsageMetricsReporter, *metrics.InmemSink, error)) {
	cases := baseCases

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			reporter, sink, err := getReporter(tcase)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}

func testUsageReporter_emitPeeringUsage_CE(t *testing.T, getMetricsReporter func(testCase) (*UsageMetricsReporter, *metrics.InmemSink, error)) {
	cases := make(map[string]testCase)
	for k, v := range baseCases {
		eg := make(map[string]metrics.GaugeValue)
		for k, v := range v.expectedGauges {
			eg[k] = v
		}
		cases[k] = testCase{v.modifyStateStore, v.getMembersFunc, eg}
	}
	peeringsCase := cases["nodes"]
	peeringsCase.modifyStateStore = func(t *testing.T, s *state.Store) {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)
		require.NoError(t, s.PeeringWrite(1, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{Name: "foo", ID: id}}))
		id, err = uuid.GenerateUUID()
		require.NoError(t, err)
		require.NoError(t, s.PeeringWrite(2, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{Name: "bar", ID: id}}))
		id, err = uuid.GenerateUUID()
		require.NoError(t, err)
		require.NoError(t, s.PeeringWrite(3, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{Name: "baz", ID: id}}))
	}
	peeringsCase.getMembersFunc = func() []serf.Member {
		return []serf.Member{
			{
				Name:   "foo",
				Tags:   map[string]string{"role": "consul"},
				Status: serf.StatusAlive,
			},
			{
				Name:   "bar",
				Tags:   map[string]string{"role": "consul"},
				Status: serf.StatusAlive,
			},
		}
	}
	peeringsCase.expectedGauges["consul.usage.test.state.nodes;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.nodes",
		Value:  0,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	peeringsCase.expectedGauges["consul.usage.test.state.peerings;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.peerings",
		Value:  3,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	peeringsCase.expectedGauges["consul.usage.test.members.clients;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.members.clients",
		Value:  0,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	cases["peerings"] = peeringsCase
	delete(cases, "nodes")

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			reporter, sink, err := getMetricsReporter(tcase)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}

func testUsageReporter_emitServiceUsage_CE(t *testing.T, getMetricsReporter func(testCase) (*UsageMetricsReporter, *metrics.InmemSink, error)) {
	cases := make(map[string]testCase)
	for k, v := range baseCases {
		eg := make(map[string]metrics.GaugeValue)
		for k, v := range v.expectedGauges {
			eg[k] = v
		}
		cases[k] = testCase{v.modifyStateStore, v.getMembersFunc, eg}
	}

	nodesAndSvcsCase := cases["nodes"]
	nodesAndSvcsCase.modifyStateStore = func(t *testing.T, s *state.Store) {
		require.NoError(t, s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
		require.NoError(t, s.EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
		require.NoError(t, s.EnsureNode(3, &structs.Node{Node: "baz", Address: "127.0.0.2"}))
		require.NoError(t, s.EnsureNode(4, &structs.Node{Node: "qux", Address: "127.0.0.3"}))

		apigw := structs.TestNodeServiceAPIGateway(t)
		apigw.ID = "api-gateway"

		mgw := structs.TestNodeServiceMeshGateway(t)
		mgw.ID = "mesh-gateway"

		tgw := structs.TestNodeServiceTerminatingGateway(t, "1.1.1.1")
		tgw.ID = "terminating-gateway"
		// Typical services and some consul services spread across two nodes
		require.NoError(t, s.EnsureService(5, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
		require.NoError(t, s.EnsureService(6, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
		require.NoError(t, s.EnsureService(7, "foo", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
		require.NoError(t, s.EnsureService(8, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
		require.NoError(t, s.EnsureService(9, "foo", &structs.NodeService{ID: "db-connect-proxy", Service: "db-connect-proxy", Tags: nil, Address: "", Port: 5000, Kind: structs.ServiceKindConnectProxy}))
		require.NoError(t, s.EnsureRegistration(10, structs.TestRegisterIngressGateway(t)))
		require.NoError(t, s.EnsureService(11, "foo", mgw))
		require.NoError(t, s.EnsureService(12, "foo", tgw))
		require.NoError(t, s.EnsureService(13, "foo", apigw))
		require.NoError(t, s.EnsureService(14, "bar", &structs.NodeService{ID: "db-native", Service: "db", Tags: nil, Address: "", Port: 5000, Connect: structs.ServiceConnect{Native: true}}))
		require.NoError(t, s.EnsureConfigEntry(15, &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "foo",
		}))
		require.NoError(t, s.EnsureConfigEntry(16, &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "bar",
		}))
		require.NoError(t, s.EnsureConfigEntry(17, &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "baz",
		}))
	}
	baseCaseMembers := nodesAndSvcsCase.getMembersFunc()
	nodesAndSvcsCase.getMembersFunc = func() []serf.Member {
		baseCaseMembers = append(baseCaseMembers, serf.Member{
			Name:   "baz",
			Tags:   map[string]string{"role": "node", "segment": "a"},
			Status: serf.StatusAlive,
		})
		baseCaseMembers = append(baseCaseMembers, serf.Member{
			Name:   "qux",
			Tags:   map[string]string{"role": "node", "segment": "b"},
			Status: serf.StatusAlive,
		})
		return baseCaseMembers
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.nodes;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.nodes",
		Value:  4,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.members.clients;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.members.clients",
		Value:  2,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.services;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.services",
		Value:  8,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.service_instances;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.service_instances",
		Value:  10,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-proxy"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "connect-proxy"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=terminating-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "terminating-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=ingress-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "ingress-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=api-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "api-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=mesh-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "mesh-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-native"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "connect-native"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.billable_service_instances;datacenter=dc1"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.billable_service_instances",
		Value: 3,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.config_entries;datacenter=dc1;kind=ingress-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.config_entries",
		Value: 3,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "ingress-gateway"},
		},
	}
	cases["nodes-and-services"] = nodesAndSvcsCase
	delete(cases, "nodes")

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			reporter, sink, err := getMetricsReporter(tcase)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}

func testUsageReporter_emitKVUsage_CE(t *testing.T, getMetricsReporter func(testCase) (*UsageMetricsReporter, *metrics.InmemSink, error)) {
	cases := make(map[string]testCase)
	for k, v := range baseCases {
		eg := make(map[string]metrics.GaugeValue)
		for k, v := range v.expectedGauges {
			eg[k] = v
		}
		cases[k] = testCase{v.modifyStateStore, v.getMembersFunc, eg}
	}

	nodesCase := cases["nodes"]
	mss := nodesCase.modifyStateStore
	nodesCase.modifyStateStore = func(t *testing.T, s *state.Store) {
		mss(t, s)
		require.NoError(t, s.KVSSet(4, &structs.DirEntry{Key: "a", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(5, &structs.DirEntry{Key: "b", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(6, &structs.DirEntry{Key: "c", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(7, &structs.DirEntry{Key: "d", Value: []byte{1}}))
		require.NoError(t, s.KVSDelete(8, "d", &acl.EnterpriseMeta{}))
		require.NoError(t, s.KVSDelete(9, "c", &acl.EnterpriseMeta{}))
		require.NoError(t, s.KVSSet(10, &structs.DirEntry{Key: "e", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(11, &structs.DirEntry{Key: "f", Value: []byte{1}}))
	}
	nodesCase.expectedGauges["consul.usage.test.state.kv_entries;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.kv_entries",
		Value:  4,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	cases["nodes"] = nodesCase

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			reporter, sink, err := getMetricsReporter(tcase)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}
