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
		getMembersFunc   getMembersFunc
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
				"consul.usage.test.consul.members.clients;datacenter=dc1": {
					Name:  "consul.usage.test.consul.members.clients",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.members.servers;datacenter=dc1": {
					Name:  "consul.usage.test.consul.members.servers",
					Value: 0,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
			},
			getMembersFunc: func() []serf.Member { return []serf.Member{} },
		},
		"nodes-and-services": {
			modfiyStateStore: func(t *testing.T, s *state.Store) {
				require.Nil(t, s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
				require.Nil(t, s.EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
				require.Nil(t, s.EnsureNode(3, &structs.Node{Node: "baz", Address: "127.0.0.2"}))
				require.Nil(t, s.EnsureNode(4, &structs.Node{Node: "qux", Address: "127.0.0.3"}))

				// Typical services and some consul services spread across two nodes
				require.Nil(t, s.EnsureService(5, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
				require.Nil(t, s.EnsureService(6, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
				require.Nil(t, s.EnsureService(7, "foo", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
				require.Nil(t, s.EnsureService(8, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
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
					{
						Name:   "baz",
						Tags:   map[string]string{"role": "node", "segment": "a"},
						Status: serf.StatusAlive,
					},
					{
						Name:   "qux",
						Tags:   map[string]string{"role": "node", "segment": "b"},
						Status: serf.StatusAlive,
					},
				}
			},
			expectedGauges: map[string]metrics.GaugeValue{
				"consul.usage.test.consul.state.nodes;datacenter=dc1": {
					Name:   "consul.usage.test.consul.state.nodes",
					Value:  4,
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
				"consul.usage.test.consul.members.clients;datacenter=dc1": {
					Name:  "consul.usage.test.consul.members.clients",
					Value: 2,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.members.servers;datacenter=dc1": {
					Name:  "consul.usage.test.consul.members.servers",
					Value: 2,
					Labels: []metrics.Label{
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.members.clients;segment=a;datacenter=dc1": {
					Name:  "consul.usage.test.consul.members.clients",
					Value: 1,
					Labels: []metrics.Label{
						{Name: "segment", Value: "a"},
						{Name: "datacenter", Value: "dc1"},
					},
				},
				"consul.usage.test.consul.members.clients;segment=b;datacenter=dc1": {
					Name:  "consul.usage.test.consul.members.clients",
					Value: 1,
					Labels: []metrics.Label{
						{Name: "segment", Value: "b"},
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

			reporter, err := NewUsageMetricsReporter(
				new(Config).
					WithStateProvider(mockStateProvider).
					WithLogger(testutil.Logger(t)).
					WithDatacenter("dc1").
					WithGetMembersFunc(tcase.getMembersFunc),
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
