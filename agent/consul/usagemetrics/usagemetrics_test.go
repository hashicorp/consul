package usagemetrics

import (
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/serf/serf"
)

type mockStateProvider struct {
	mock.Mock
}

func (m *mockStateProvider) State() *state.Store {
	retValues := m.Called()
	return retValues.Get(0).(*state.Store)
}

func TestUsageReporter_Run_Nodes(t *testing.T) {
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
			},
			getMembersFunc: func() []serf.Member { return []serf.Member{} },
		},
		"nodes": {
			modfiyStateStore: func(t *testing.T, s *state.Store) {
				require.Nil(t, s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
				require.Nil(t, s.EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
				require.Nil(t, s.EnsureNode(3, &structs.Node{Node: "baz", Address: "127.0.0.2"}))
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
						Tags:   map[string]string{"role": "node"},
						Status: serf.StatusAlive,
					},
				}
			},
			expectedGauges: map[string]metrics.GaugeValue{
				"consul.usage.test.consul.state.nodes;datacenter=dc1": {
					Name:   "consul.usage.test.consul.state.nodes",
					Value:  3,
					Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
				},
				"consul.usage.test.consul.members.clients;datacenter=dc1": {
					Name:   "consul.usage.test.consul.members.clients",
					Value:  1,
					Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
				},
				"consul.usage.test.consul.members.servers;datacenter=dc1": {
					Name:   "consul.usage.test.consul.members.servers",
					Value:  2,
					Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
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
			s, err := newStateStore()
			require.NoError(t, err)
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

			// Range over the expected values instead of just doing an Equal
			// comparison on the maps because of different metrics emitted between
			// OSS and Ent. The enterprise and OSS tests have a full equality
			// comparison on the maps.
			for key, expected := range tcase.expectedGauges {
				require.Equal(t, expected, intv.Gauges[key])
			}
		})
	}
}
