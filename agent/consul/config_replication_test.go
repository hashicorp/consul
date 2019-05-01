package consul

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestReplication_ConfigEntries(t *testing.T) {
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
		_, remote, err := s1.fsm.State().ConfigEntries(nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ConfigEntries(nil)
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

		var out struct{}
		require.NoError(t, s1.RPC("ConfigEntry.Delete", &arg, &out))
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})
}
