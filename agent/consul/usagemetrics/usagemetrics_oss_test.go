// +build !consulent

package usagemetrics

import (
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/serf/serf"
)

func newStateStore() (*state.Store, error) {
	return state.NewStateStore(nil), nil
}

func TestUsageReporter_emitServiceUsage_OSS(t *testing.T) {
	type testCase struct {
		modfiyStateStore func(t *testing.T, s *state.Store)
		modifySerf       func(t *testing.T, s *serf.Serf)
		expectedGauges   map[string]metrics.GaugeValue
	}
	cases := map[string]testCase{
		"empty-state": {
			expectedGauges: map[string]metrics.GaugeValue{
				"consul.usage.test.consul.state.nodes;datacenter=dc1": {
					Name:   "consul.usage.test.consul.state.nodes",
					Value:  0,
					Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
				},
				"consul.usage.test.consul.state.services;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.services",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.state.service_instances;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.service_instances",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.state.client_agents;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.client_agents",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.state.server_agents;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.server_agents",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
			},
		},
		"nodes-and-services": {
			modfiyStateStore: func(t *testing.T, s *state.Store) {
				require.Nil(t, s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
				require.Nil(t, s.EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
				require.Nil(t, s.EnsureNode(3, &structs.Node{Node: "baz", Address: "127.0.0.2"}))

				// Typical services and some consul services spread across two nodes
				require.Nil(t, s.EnsureService(4, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
				require.Nil(t, s.EnsureService(5, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
				require.Nil(t, s.EnsureService(6, "foo", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
				require.Nil(t, s.EnsureService(7, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
			},
			modifySerf: func(t *testing.T, s *serf.Serf) {
				// TODO: How can we add members to serf here?
			},
			expectedGauges: map[string]metrics.GaugeValue{
				"consul.usage.test.consul.state.nodes;datacenter=dc1": {
					Name:   "consul.usage.test.consul.state.nodes",
					Value:  3,
					Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
				},
				"consul.usage.test.consul.state.services;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.services",
					Value: 3,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.state.service_instances;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.service_instances",
					Value: 4,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.state.client_agents;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.client_agents",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.state.server_agents;datacenter=dc1": {
					Name:  "consul.usage.test.consul.state.server_agents",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			// Only have a single interval for the test
			sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
			cfg := metrics.DefaultConfig("consul.usage.test")
			cfg.EnableHostname = false
			metrics.NewGlobal(cfg, sink)

			mockStateProvider := &mockStateProvider{}
			s := state.NewStateStore(nil)
			if tcase.modfiyStateStore != nil {
				tcase.modfiyStateStore(t, s)
			}
			mockStateProvider.On("State").Return(s)

			// Passes but we can't test the code changes if we do this
			// srf := &serf.Serf{}

			// Failed to create memberlist: Could not set up network transport:
			// failed to obtain an address: Failed to start TCP listener on
			// "0.0.0.0" port 7946: listen tcp 0.0.0.0:7946: bind: address
			// already in use
			serfConf := serf.DefaultConfig()
			srf, err := serf.Create(serfConf)
			require.NoError(t, err)
			if tcase.modifySerf != nil {
				tcase.modifySerf(t, srf)
			}

			reporter, err := NewUsageMetricsReporter(
				new(Config).
					WithStateProvider(mockStateProvider).
					WithLogger(testutil.Logger(t)).
					WithDatacenter("dc1"),
				srf,
			)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			require.Equal(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}
