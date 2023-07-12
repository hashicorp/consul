package consul

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestReplication_ConfigSort(t *testing.T) {
	newDefaults := func(name, protocol string) *structs.ServiceConfigEntry {
		return &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     name,
			Protocol: protocol,
		}
	}
	newResolver := func(name string, timeout time.Duration) *structs.ServiceResolverConfigEntry {
		return &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           name,
			ConnectTimeout: timeout,
		}
	}

	type testcase struct {
		configs []structs.ConfigEntry
		expect  []structs.ConfigEntry
	}

	cases := map[string]testcase{
		"none": {},
		"one": {
			configs: []structs.ConfigEntry{
				newDefaults("web", "grpc"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("web", "grpc"),
			},
		},
		"just kinds": {
			configs: []structs.ConfigEntry{
				newResolver("web", 33*time.Second),
				newDefaults("web", "grpc"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("web", "grpc"),
				newResolver("web", 33*time.Second),
			},
		},
		"just names": {
			configs: []structs.ConfigEntry{
				newDefaults("db", "grpc"),
				newDefaults("api", "http2"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("api", "http2"),
				newDefaults("db", "grpc"),
			},
		},
		"all": {
			configs: []structs.ConfigEntry{
				newResolver("web", 33*time.Second),
				newDefaults("web", "grpc"),
				newDefaults("db", "grpc"),
				newDefaults("api", "http2"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("api", "http2"),
				newDefaults("db", "grpc"),
				newDefaults("web", "grpc"),
				newResolver("web", 33*time.Second),
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			configSort(tc.configs)
			require.Equal(t, tc.expect, tc.configs)
			// and it should be stable
			configSort(tc.configs)
			require.Equal(t, tc.expect, tc.configs)
		})
	}
}

func TestReplication_ConfigEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ConfigReplicationRate = 100
		c.ConfigReplicationBurst = 100
		c.ConfigReplicationApplyLimit = 1000000
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Create some new configuration entries
	var entries []structs.ConfigEntry
	for i := 0; i < 50; i++ {
		arg := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Op:         structs.ConfigEntryUpsert,
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     fmt.Sprintf("svc-%d", i),
				Protocol: "tcp",
			},
		}

		out := false
		require.NoError(t, s1.RPC("ConfigEntry.Apply", &arg, &out))
		entries = append(entries, arg.Entry)
	}

	arg := structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Op:         structs.ConfigEntryUpsert,
		Entry: &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: "global",
			Config: map[string]interface{}{
				"foo": "bar",
				"bar": 1,
			},
		},
	}

	out := false
	require.NoError(t, s1.RPC("ConfigEntry.Apply", &arg, &out))
	entries = append(entries, arg.Entry)

	checkSame := func(t *retry.R) error {
		_, remote, err := s1.fsm.State().ConfigEntries(nil, structs.ReplicationEnterpriseMeta())
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ConfigEntries(nil, structs.ReplicationEnterpriseMeta())
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i, entry := range remote {
			require.Equal(t, entry.GetKind(), local[i].GetKind())
			require.Equal(t, entry.GetName(), local[i].GetName())

			// more validations
			switch entry.GetKind() {
			case structs.ServiceDefaults:
				localSvc, ok := local[i].(*structs.ServiceConfigEntry)
				require.True(t, ok)
				remoteSvc, ok := entry.(*structs.ServiceConfigEntry)
				require.True(t, ok)

				require.Equal(t, remoteSvc.Protocol, localSvc.Protocol)
			case structs.ProxyDefaults:
				localProxy, ok := local[i].(*structs.ProxyConfigEntry)
				require.True(t, ok)
				remoteProxy, ok := entry.(*structs.ProxyConfigEntry)
				require.True(t, ok)

				require.Equal(t, remoteProxy.Config, localProxy.Config)
			}
		}
		return nil
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Update those policies
	for i := 0; i < 50; i++ {
		arg := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Op:         structs.ConfigEntryUpsert,
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     fmt.Sprintf("svc-%d", i),
				Protocol: "udp",
			},
		}

		out := false
		require.NoError(t, s1.RPC("ConfigEntry.Apply", &arg, &out))
	}

	arg = structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Op:         structs.ConfigEntryUpsert,
		Entry: &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: "global",
			Config: map[string]interface{}{
				"foo": "baz",
				"baz": 2,
			},
		},
	}

	require.NoError(t, s1.RPC("ConfigEntry.Apply", &arg, &out))

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	for _, entry := range entries {
		arg := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Op:         structs.ConfigEntryDelete,
			Entry:      entry,
		}

		var out structs.ConfigEntryDeleteResponse
		require.NoError(t, s1.RPC("ConfigEntry.Delete", &arg, &out))
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})
}

func TestReplication_ConfigEntries_GraphValidationErrorDuringReplication(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
	})
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ConfigReplicationRate = 100
		c.ConfigReplicationBurst = 100
		c.ConfigReplicationApplyLimit = 1000000
	})
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Create two entries that will replicate in the wrong order and not work.
	entries := []structs.ConfigEntry{
		&structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "foo",
			Protocol: "http",
		},
		&structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "foo",
			Listeners: []structs.IngressListener{
				{
					Port:     9191,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name: "foo",
						},
					},
				},
			},
		},
	}
	for _, entry := range entries {
		arg := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Op:         structs.ConfigEntryUpsert,
			Entry:      entry,
		}

		out := false
		require.NoError(t, s1.RPC("ConfigEntry.Apply", &arg, &out))
	}

	// Try to join which should kick off replication.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	checkSame := func(t require.TestingT) error {
		_, remote, err := s1.fsm.State().ConfigEntries(nil, structs.ReplicationEnterpriseMeta())
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ConfigEntries(nil, structs.ReplicationEnterpriseMeta())
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i, entry := range remote {
			require.Equal(t, entry.GetKind(), local[i].GetKind())
			require.Equal(t, entry.GetName(), local[i].GetName())

			// more validations
			switch entry.GetKind() {
			case structs.IngressGateway:
				localGw, ok := local[i].(*structs.IngressGatewayConfigEntry)
				require.True(t, ok)
				remoteGw, ok := entry.(*structs.IngressGatewayConfigEntry)
				require.True(t, ok)
				require.Len(t, remoteGw.Listeners, 1)
				require.Len(t, localGw.Listeners, 1)
				require.Equal(t, remoteGw.Listeners[0].Protocol, localGw.Listeners[0].Protocol)
			case structs.ServiceDefaults:
				localSvc, ok := local[i].(*structs.ServiceConfigEntry)
				require.True(t, ok)
				remoteSvc, ok := entry.(*structs.ServiceConfigEntry)
				require.True(t, ok)
				require.Equal(t, remoteSvc.Protocol, localSvc.Protocol)
			}
		}
		return nil
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})
}
