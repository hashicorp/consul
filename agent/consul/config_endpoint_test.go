package consul

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestConfigEntry_Apply(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	joinWAN(t, s2, s1)
	// wait for cross-dc queries to work
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	testutil.RunStep(t, "send the apply request to dc2 - it should get forwarded to dc1", func(t *testing.T) {
		updated := &structs.ServiceConfigEntry{
			Name: "foo",
		}
		args := structs.ConfigEntryRequest{
			Datacenter: "dc2",
			Entry:      updated,
		}
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec2, "ConfigEntry.Apply", &args, &out))
		require.True(t, out)
	})

	var originalModifyIndex uint64
	testutil.RunStep(t, "verify the entry was updated in the primary and secondary", func(t *testing.T) {
		// the previous RPC should not return until the primary has been updated but will return
		// before the secondary has the data.
		_, entry, err := s1.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
		require.NoError(t, err)

		serviceConf, ok := entry.(*structs.ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, "foo", serviceConf.Name)
		require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)

		retry.Run(t, func(r *retry.R) {
			// wait for replication to happen
			_, entry, err := s2.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
			require.NoError(r, err)
			require.NotNil(r, entry)
			// this test is not testing that the config entries that are replicated are correct as thats done elsewhere.
		})
		originalModifyIndex = serviceConf.ModifyIndex
	})

	testutil.RunStep(t, "update the entry again in the primary", func(t *testing.T) {
		updated := &structs.ServiceConfigEntry{
			Name: "foo",
			MeshGateway: structs.MeshGatewayConfig{
				Mode: structs.MeshGatewayModeLocal,
			},
		}

		args := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Op:         structs.ConfigEntryUpsertCAS,
			Entry:      updated,
		}

		testutil.RunStep(t, "with the wrong CAS", func(t *testing.T) {
			var out bool
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
			require.False(t, out)
		})
		testutil.RunStep(t, "with the correct CAS", func(t *testing.T) {
			var out bool
			args.Entry.GetRaftIndex().ModifyIndex = originalModifyIndex
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
			require.True(t, out)
		})
	})

	testutil.RunStep(t, "verify the entry was updated in the state store", func(t *testing.T) {
		_, entry, err := s1.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
		require.NoError(t, err)

		serviceConf, ok := entry.(*structs.ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)
		require.Equal(t, "foo", serviceConf.Name)
		require.Equal(t, "", serviceConf.Protocol)
		require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)
	})

	testutil.RunStep(t, "verify no-op updates do not advance the raft indexes", func(t *testing.T) {
		var modifyIndex uint64
		for i := 0; i < 3; i++ {
			testutil.RunStep(t, fmt.Sprintf("iteration %d", i), func(t *testing.T) {
				args := structs.ConfigEntryRequest{
					Datacenter: "dc1",
					Op:         structs.ConfigEntryUpsert,
					Entry: &structs.ServiceConfigEntry{
						Kind:     structs.ServiceDefaults,
						Name:     "noop",
						Protocol: "grpc",
					},
				}
				var out bool
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
				require.True(t, out)

				getIndex, entry, err := s1.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "noop", nil)
				require.NoError(t, err)
				require.NotNil(t, entry)

				listIndex, entries, err := s1.fsm.State().ConfigEntries(nil, nil)
				require.NoError(t, err)
				require.Len(t, entries, 2)

				if i == 0 {
					modifyIndex = entry.GetRaftIndex().ModifyIndex
				} else {
					require.Equal(t, modifyIndex, entry.GetRaftIndex().ModifyIndex)
					require.Equal(t, modifyIndex, getIndex)
					require.Equal(t, modifyIndex, listIndex)
				}
			})
		}
	})
}

func TestConfigEntry_ProxyDefaultsMeshGateway(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	args := structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Entry: &structs.ProxyConfigEntry{
			Kind:        "proxy-defaults",
			Name:        "global",
			MeshGateway: structs.MeshGatewayConfig{Mode: "local"},
		},
	}
	out := false
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
	require.True(t, out)

	state := s1.fsm.State()
	_, entry, err := state.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)

	proxyConf, ok := entry.(*structs.ProxyConfigEntry)
	require.True(t, ok)
	require.Equal(t, structs.MeshGatewayModeLocal, proxyConf.MeshGateway.Mode)
}

func TestConfigEntry_Apply_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
service "foo" {
	policy = "write"
}
operator = "write"
`
	id := createToken(t, codec, rules)

	// This should fail since we don't have write perms for the "db" service.
	args := structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Entry: &structs.ServiceConfigEntry{
			Name: "db",
		},
		WriteRequest: structs.WriteRequest{Token: id},
	}
	out := false
	err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The "foo" service should work.
	args.Entry = &structs.ServiceConfigEntry{
		Name: "foo",
	}
	err = msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out)
	require.NoError(t, err)

	state := s1.fsm.State()
	_, entry, err := state.ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
	require.NoError(t, err)

	serviceConf, ok := entry.(*structs.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, "foo", serviceConf.Name)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)

	// Try to update the global proxy args with the anonymous token - this should fail.
	proxyArgs := structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Entry: &structs.ProxyConfigEntry{
			Config: map[string]interface{}{
				"foo": 1,
			},
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &proxyArgs, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Now with the privileged token.
	proxyArgs.WriteRequest.Token = id
	err = msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &proxyArgs, &out)
	require.NoError(t, err)
}

func TestConfigEntry_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create a dummy service in the state store to look up.
	entry := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, entry))

	args := structs.ConfigEntryQuery{
		Kind:       structs.ServiceDefaults,
		Name:       "foo",
		Datacenter: s1.config.Datacenter,
	}
	var out structs.ConfigEntryResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Get", &args, &out))

	serviceConf, ok := out.Entry.(*structs.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, "foo", serviceConf.Name)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)
}

func TestConfigEntry_Get_BlockOnNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
	})

	codec := rpcClient(t, s1)

	{ // create one relevant entry
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Entry: &structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "alpha",
			},
		}, &out))
		require.True(t, out)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := structs.ConfigEntryQuery{
					Kind: structs.ServiceDefaults,
					Name: "does-not-exist",
				}
				args.QueryOptions.MinQueryIndex = minQueryIndex

				var out structs.ConfigEntryResponse
				errCh := channelCallRPC(s1, "ConfigEntry.Get", &args, &out, nil)
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out bool
				return channelCallRPC(s1, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
					Entry: &structs.ServiceConfigEntry{
						Kind: structs.ServiceDefaults,
						Name: fmt.Sprintf("other%d", i),
					},
				}, &out, func() error {
					if !out {
						return fmt.Errorf("[%d] unexpectedly returned false", i)
					}
					return nil
				})
			},
		)
	})
}

func TestConfigEntry_Get_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
service "foo" {
	policy = "read"
}
operator = "read"
`
	id := createToken(t, codec, rules)

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))

	// This should fail since we don't have write perms for the "db" service.
	args := structs.ConfigEntryQuery{
		Kind:         structs.ServiceDefaults,
		Name:         "db",
		Datacenter:   s1.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var out structs.ConfigEntryResponse
	err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.Get", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The "foo" service should work.
	args.Name = "foo"
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Get", &args, &out))

	serviceConf, ok := out.Entry.(*structs.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, "foo", serviceConf.Name)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)
}

func TestConfigEntry_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create some dummy services in the state store to look up.
	state := s1.fsm.State()
	expected := structs.IndexedConfigEntries{
		Entries: []structs.ConfigEntry{
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "bar",
			},
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "foo",
			},
		},
	}
	require.NoError(t, state.EnsureConfigEntry(1, expected.Entries[0]))
	require.NoError(t, state.EnsureConfigEntry(2, expected.Entries[1]))

	args := structs.ConfigEntryQuery{
		Kind:       structs.ServiceDefaults,
		Datacenter: "dc1",
	}
	var out structs.IndexedConfigEntries
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.List", &args, &out))

	expected.Kind = structs.ServiceDefaults
	expected.QueryMeta = out.QueryMeta
	require.Equal(t, expected, out)
}

func TestConfigEntry_List_BlockOnNoChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
	})

	codec := rpcClient(t, s1)

	run := func(t *testing.T, dataPrefix string) {
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := structs.ConfigEntryQuery{
					Kind:       structs.ServiceDefaults,
					Datacenter: "dc1",
				}
				args.QueryOptions.MinQueryIndex = minQueryIndex

				var out structs.IndexedConfigEntries

				errCh := channelCallRPC(s1, "ConfigEntry.List", &args, &out, nil)
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out bool
				return channelCallRPC(s1, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
					Entry: &structs.ServiceResolverConfigEntry{
						Kind:           structs.ServiceResolver,
						Name:           fmt.Sprintf(dataPrefix+"%d", i),
						ConnectTimeout: 33 * time.Second,
					},
				}, &out, func() error {
					if !out {
						return fmt.Errorf("[%d] unexpectedly returned false", i)
					}
					return nil
				})
			},
		)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
		run(t, "other")
	})

	{ // Create some dummy services in the state store to look up.
		for _, entry := range []structs.ConfigEntry{
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "bar",
			},
			&structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "foo",
			},
		} {
			var out bool
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
				Entry: entry,
			}, &out))
			require.True(t, out)
		}
	}

	testutil.RunStep(t, "test the errNotChanged path", func(t *testing.T) {
		run(t, "completely-different-other")
	})
}

func TestConfigEntry_ListAll(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create some dummy services in the state store to look up.
	state := s1.fsm.State()
	entries := []structs.ConfigEntry{
		&structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: "global",
		},
		&structs.ServiceConfigEntry{
			Kind: structs.ServiceDefaults,
			Name: "bar",
		},
		&structs.ServiceConfigEntry{
			Kind: structs.ServiceDefaults,
			Name: "foo",
		},
		&structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "api",
			Sources: []*structs.SourceIntention{
				{
					Name:   "web",
					Action: structs.IntentionActionAllow,
				},
			},
		},
	}
	require.NoError(t, state.EnsureConfigEntry(1, entries[0]))
	require.NoError(t, state.EnsureConfigEntry(2, entries[1]))
	require.NoError(t, state.EnsureConfigEntry(3, entries[2]))
	require.NoError(t, state.EnsureConfigEntry(4, entries[3]))

	t.Run("all kinds", func(t *testing.T) {
		args := structs.ConfigEntryListAllRequest{
			Datacenter: "dc1",
			Kinds:      structs.AllConfigEntryKinds,
		}
		var out structs.IndexedGenericConfigEntries
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ListAll", &args, &out))

		expected := structs.IndexedGenericConfigEntries{
			Entries:   entries[:],
			QueryMeta: out.QueryMeta,
		}
		require.Equal(t, expected, out)
	})

	t.Run("all kinds pre 1.9.0", func(t *testing.T) {
		args := structs.ConfigEntryListAllRequest{
			Datacenter: "dc1",
			Kinds:      nil, // let it default
		}
		var out structs.IndexedGenericConfigEntries
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ListAll", &args, &out))

		expected := structs.IndexedGenericConfigEntries{
			Entries:   entries[0:3],
			QueryMeta: out.QueryMeta,
		}
		require.Equal(t, expected, out)
	})

	t.Run("omit service defaults", func(t *testing.T) {
		args := structs.ConfigEntryListAllRequest{
			Datacenter: "dc1",
			Kinds: []string{
				structs.ProxyDefaults,
			},
		}
		var out structs.IndexedGenericConfigEntries
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ListAll", &args, &out))

		expected := structs.IndexedGenericConfigEntries{
			Entries:   entries[0:1],
			QueryMeta: out.QueryMeta,
		}

		require.Equal(t, expected, out)
	})
}

func TestConfigEntry_List_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
service "foo" {
	policy = "read"
}
operator = "read"
`
	id := createToken(t, codec, rules)

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(t, state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "db",
	}))

	// This should filter out the "db" service since we don't have permissions for it.
	args := structs.ConfigEntryQuery{
		Kind:         structs.ServiceDefaults,
		Datacenter:   s1.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var out structs.IndexedConfigEntries
	err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.List", &args, &out)
	require.NoError(t, err)

	serviceConf, ok := out.Entries[0].(*structs.ServiceConfigEntry)
	require.Len(t, out.Entries, 1)
	require.True(t, ok)
	require.Equal(t, "foo", serviceConf.Name)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)
	require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")

	// Get the global proxy config.
	args.Kind = structs.ProxyDefaults
	err = msgpackrpc.CallWithCodec(codec, "ConfigEntry.List", &args, &out)
	require.NoError(t, err)

	proxyConf, ok := out.Entries[0].(*structs.ProxyConfigEntry)
	require.Len(t, out.Entries, 1)
	require.True(t, ok)
	require.Equal(t, structs.ProxyConfigGlobal, proxyConf.Name)
	require.Equal(t, structs.ProxyDefaults, proxyConf.Kind)
	require.False(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
}

func TestConfigEntry_ListAll_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
service "foo" {
	policy = "read"
}
operator = "read"
`
	id := createToken(t, codec, rules)

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(t, state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "db",
	}))

	// This should filter out the "db" service since we don't have permissions for it.
	args := structs.ConfigEntryListAllRequest{
		Datacenter:   s1.config.Datacenter,
		Kinds:        structs.AllConfigEntryKinds,
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var out structs.IndexedGenericConfigEntries
	err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.ListAll", &args, &out)
	require.NoError(t, err)
	require.Len(t, out.Entries, 2)
	svcIndex := 0
	proxyIndex := 1
	if out.Entries[0].GetKind() == structs.ProxyDefaults {
		svcIndex = 1
		proxyIndex = 0
	}

	svcConf, ok := out.Entries[svcIndex].(*structs.ServiceConfigEntry)
	require.True(t, ok)
	proxyConf, ok := out.Entries[proxyIndex].(*structs.ProxyConfigEntry)
	require.True(t, ok)

	require.Equal(t, "foo", svcConf.Name)
	require.Equal(t, structs.ServiceDefaults, svcConf.Kind)
	require.Equal(t, structs.ProxyConfigGlobal, proxyConf.Name)
	require.Equal(t, structs.ProxyDefaults, proxyConf.Kind)
	require.True(t, out.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
}

func TestConfigEntry_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec2 := rpcClient(t, s2)
	defer codec2.Close()

	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	joinWAN(t, s2, s1)
	// wait for cross-dc queries to work
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	testutil.RunStep(t, "create a dummy service in the state store to look up", func(t *testing.T) {
		entry := &structs.ServiceConfigEntry{
			Kind: structs.ServiceDefaults,
			Name: "foo",
		}
		require.NoError(t, s1.fsm.State().EnsureConfigEntry(1, entry))
	})

	testutil.RunStep(t, "verify it exists in the primary and is replicated to the secondary", func(t *testing.T) {
		// Verify it's there.
		_, existing, err := s1.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
		require.NoError(t, err)

		serviceConf, ok := existing.(*structs.ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, "foo", serviceConf.Name)
		require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)

		retry.Run(t, func(r *retry.R) {
			// wait for it to be replicated into the secondary dc
			_, existing, err := s2.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
			require.NoError(r, err)
			require.NotNil(r, existing)
		})
	})

	testutil.RunStep(t, "send the delete request to dc2 - it should get forwarded to dc1", func(t *testing.T) {
		args := structs.ConfigEntryRequest{
			Datacenter: "dc2",
			Entry: &structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "foo",
			},
		}
		var out structs.ConfigEntryDeleteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec2, "ConfigEntry.Delete", &args, &out))
		require.True(t, out.Deleted)
	})

	testutil.RunStep(t, "verify the entry was deleted in the primary and secondary", func(t *testing.T) {
		// Verify the entry was deleted.
		_, existing, err := s1.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
		require.NoError(t, err)
		require.Nil(t, existing)

		// verify it gets deleted from the secondary too
		retry.Run(t, func(r *retry.R) {
			_, existing, err := s2.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
			require.NoError(r, err)
			require.Nil(r, existing)
		})
	})

	testutil.RunStep(t, "delete in dc1 again - should be fine", func(t *testing.T) {
		args := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "foo",
			},
		}
		var out structs.ConfigEntryDeleteResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &out))
		require.True(t, out.Deleted)
	})
}

func TestConfigEntry_DeleteCAS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()

	codec := rpcClient(t, s)
	defer codec.Close()

	testrpc.WaitForLeader(t, s.RPC, "dc1")

	// Create a simple config entry.
	entry := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}
	state := s.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, entry))

	// Verify it's there.
	_, existing, err := state.ConfigEntry(nil, entry.Kind, entry.Name, nil)
	require.NoError(t, err)

	// Send a delete CAS request with an invalid index.
	args := structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Op:         structs.ConfigEntryDeleteCAS,
	}
	args.Entry = entry.Clone()
	args.Entry.GetRaftIndex().ModifyIndex = existing.GetRaftIndex().ModifyIndex - 1

	var rsp structs.ConfigEntryDeleteResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &rsp))
	require.False(t, rsp.Deleted)

	// Verify the entry was not deleted.
	_, existing, err = s.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
	require.NoError(t, err)
	require.NotNil(t, existing)

	// Restore the valid index and try again.
	args.Entry.GetRaftIndex().ModifyIndex = existing.GetRaftIndex().ModifyIndex

	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &rsp))
	require.True(t, rsp.Deleted)

	// Verify the entry was deleted.
	_, existing, err = s.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
	require.NoError(t, err)
	require.Nil(t, existing)
}

func TestConfigEntry_Delete_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
service "foo" {
	policy = "write"
}
operator = "write"
`
	id := createToken(t, codec, rules)

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))

	// This should fail since we don't have write perms for the "db" service.
	args := structs.ConfigEntryRequest{
		Datacenter: s1.config.Datacenter,
		Entry: &structs.ServiceConfigEntry{
			Name: "db",
		},
		WriteRequest: structs.WriteRequest{Token: id},
	}
	var out struct{}
	err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The "foo" service should work.
	args.Entry = &structs.ServiceConfigEntry{
		Name: "foo",
	}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &out))

	// Verify the entry was deleted.
	_, existing, err := state.ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
	require.NoError(t, err)
	require.Nil(t, existing)

	// Try to delete the global proxy config without a token.
	args = structs.ConfigEntryRequest{
		Datacenter: s1.config.Datacenter,
		Entry: &structs.ProxyConfigEntry{
			Name: structs.ProxyConfigGlobal,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Now delete with a valid token.
	args.WriteRequest.Token = id
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &out))

	_, existing, err = state.ConfigEntry(nil, structs.ServiceDefaults, "foo", nil)
	require.NoError(t, err)
	require.Nil(t, existing)
}

func TestConfigEntry_ResolveServiceConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create a dummy proxy/service config in the state store to look up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"foo": 1,
		},
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "foo",
		Protocol: "http",
		Meta:     map[string]string{"foo": "bar"},
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "bar",
		Protocol: "grpc",
	}))

	args := structs.ServiceConfigRequest{
		Name:       "foo",
		Datacenter: s1.config.Datacenter,
		Upstreams:  []string{"bar", "baz"},
	}
	var out structs.ServiceConfigResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

	expected := structs.ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"foo":      int64(1),
			"protocol": "http",
		},
		UpstreamConfigs: map[string]map[string]interface{}{
			"bar": {
				"protocol": "grpc",
			},
		},
		Meta: map[string]string{"foo": "bar"},
		// Don't know what this is deterministically
		QueryMeta: out.QueryMeta,
	}
	require.Equal(t, expected, out)

	_, entry, err := s1.fsm.State().ConfigEntry(nil, structs.ProxyDefaults, structs.ProxyConfigGlobal, nil)
	require.NoError(t, err)
	require.NotNil(t, entry)
	proxyConf, ok := entry.(*structs.ProxyConfigEntry)
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{"foo": 1}, proxyConf.Config)
}

func TestConfigEntry_ResolveServiceConfig_TransparentProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tt := []struct {
		name     string
		entries  []structs.ConfigEntry
		request  structs.ServiceConfigRequest
		proxyCfg structs.ConnectProxyConfig
		expect   structs.ServiceConfigResponse
	}{
		{
			name: "from proxy-defaults",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
			},
			expect: structs.ServiceConfigResponse{
				Mode: structs.ProxyModeTransparent,
				TransparentProxy: structs.TransparentProxyConfig{
					OutboundListenerPort: 10101,
					DialedDirectly:       true,
				},
			},
		},
		{
			name: "from service-defaults",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind:             structs.ServiceDefaults,
					Name:             "foo",
					Mode:             structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{OutboundListenerPort: 808},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
			},
			expect: structs.ServiceConfigResponse{
				Mode:             structs.ProxyModeTransparent,
				TransparentProxy: structs.TransparentProxyConfig{OutboundListenerPort: 808},
			},
		},
		{
			name: "from service-defaults with endpoint",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "foo",
					Mode: structs.ProxyModeTransparent,
					Destination: &structs.DestinationConfig{
						Addresses: []string{"hello.world.com"},
						Port:      443,
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
			},
			expect: structs.ServiceConfigResponse{
				Mode: structs.ProxyModeTransparent,
				Destination: structs.DestinationConfig{
					Addresses: []string{"hello.world.com"},
					Port:      443,
				},
			},
		},
		{
			name: "service-defaults overrides proxy-defaults",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Mode: structs.ProxyModeDirect,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       false,
					},
				},
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "foo",
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 808,
						DialedDirectly:       true,
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
			},
			expect: structs.ServiceConfigResponse{
				Mode: structs.ProxyModeTransparent,
				TransparentProxy: structs.TransparentProxyConfig{
					OutboundListenerPort: 808,
					DialedDirectly:       true,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dir1, s1 := testServer(t)
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()

			codec := rpcClient(t, s1)
			defer codec.Close()

			// Boostrap the config entries
			idx := uint64(1)
			for _, conf := range tc.entries {
				require.NoError(t, s1.fsm.State().EnsureConfigEntry(idx, conf))
				idx++
			}

			var out structs.ServiceConfigResponse
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &tc.request, &out))

			// Don't know what this is deterministically, so we grab it from the response
			tc.expect.QueryMeta = out.QueryMeta

			require.Equal(t, tc.expect, out)
		})
	}
}

func TestConfigEntry_ResolveServiceConfig_Upstreams(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	mysql := structs.NewServiceID("mysql", structs.DefaultEnterpriseMetaInDefaultPartition())
	cache := structs.NewServiceID("cache", structs.DefaultEnterpriseMetaInDefaultPartition())
	wildcard := structs.NewServiceID(structs.WildcardSpecifier, structs.WildcardEnterpriseMetaInDefaultPartition())

	tt := []struct {
		name    string
		entries []structs.ConfigEntry
		request structs.ServiceConfigRequest
		expect  structs.ServiceConfigResponse
	}{
		{
			name: "upstream config entries from Upstreams and service-defaults",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "grpc",
					},
				},
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "api",
					UpstreamConfig: &structs.UpstreamConfiguration{
						Overrides: []*structs.UpstreamConfig{
							{
								Name:     "mysql",
								Protocol: "http",
							},
						},
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "api",
				Datacenter: "dc1",
				Upstreams:  []string{"cache"},
			},
			expect: structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{
					"protocol": "grpc",
				},
				UpstreamConfigs: map[string]map[string]interface{}{
					"mysql": {
						"protocol": "http",
					},
					"cache": {
						"protocol": "grpc",
					},
				},
			},
		},
		{
			name: "upstream config entries from UpstreamIDs and service-defaults",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "grpc",
					},
				},
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "api",
					UpstreamConfig: &structs.UpstreamConfiguration{
						Overrides: []*structs.UpstreamConfig{
							{
								Name:     "mysql",
								Protocol: "http",
							},
						},
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "api",
				Datacenter: "dc1",
				UpstreamIDs: []structs.ServiceID{
					cache,
				},
			},
			expect: structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{
					"protocol": "grpc",
				},
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: cache,
						Config: map[string]interface{}{
							"protocol": "grpc",
						},
					},
					{
						Upstream: structs.ServiceID{
							ID:             "mysql",
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
						},
						Config: map[string]interface{}{
							"protocol": "http",
						},
					},
				},
			},
		},
		{
			name: "proxy registration overrides upstream_defaults",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "api",
					UpstreamConfig: &structs.UpstreamConfiguration{
						Defaults: &structs.UpstreamConfig{
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
						},
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "api",
				Datacenter: "dc1",
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeNone,
				},
				UpstreamIDs: []structs.ServiceID{
					mysql,
				},
			},
			expect: structs.ServiceConfigResponse{
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": map[string]interface{}{
								"Mode": "remote",
							},
						},
					},
					{
						Upstream: mysql,
						Config: map[string]interface{}{
							"mesh_gateway": map[string]interface{}{
								"Mode": "none",
							},
						},
					},
				},
			},
		},
		{
			name: "upstream_config.overrides override all",
			entries: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind: structs.ProxyDefaults,
					Name: structs.ProxyConfigGlobal,
					Config: map[string]interface{}{
						"protocol": "udp",
					},
				},
				&structs.ServiceConfigEntry{
					Kind:     structs.ServiceDefaults,
					Name:     "api",
					Protocol: "tcp",
				},
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "api",
					UpstreamConfig: &structs.UpstreamConfiguration{
						Defaults: &structs.UpstreamConfig{
							Protocol:    "http",
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
							PassiveHealthCheck: &structs.PassiveHealthCheck{
								Interval:                10,
								MaxFailures:             2,
								EnforcingConsecutive5xx: uintPointer(60),
							},
						},
						Overrides: []*structs.UpstreamConfig{
							{
								Name:        "mysql",
								Protocol:    "grpc",
								MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal},
							},
						},
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "api",
				Datacenter: "dc1",
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeNone,
				},
				UpstreamIDs: []structs.ServiceID{
					mysql,
				},
			},
			expect: structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{
					"protocol": "udp",
				},
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"passive_health_check": map[string]interface{}{
								"Interval":                int64(10),
								"MaxFailures":             int64(2),
								"EnforcingConsecutive5xx": int64(60),
							},
							"mesh_gateway": map[string]interface{}{
								"Mode": "remote",
							},
							"protocol": "http",
						},
					},
					{
						Upstream: mysql,
						Config: map[string]interface{}{
							"passive_health_check": map[string]interface{}{
								"Interval":                int64(10),
								"MaxFailures":             int64(2),
								"EnforcingConsecutive5xx": int64(60),
							},
							"mesh_gateway": map[string]interface{}{
								"Mode": "local",
							},
							"protocol": "grpc",
						},
					},
				},
			},
		},
		{
			name: "without upstream args we should return centralized config with tproxy arg",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "api",
					UpstreamConfig: &structs.UpstreamConfiguration{
						Defaults: &structs.UpstreamConfig{
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
						},
						Overrides: []*structs.UpstreamConfig{
							{
								Name:     "mysql",
								Protocol: "grpc",
							},
						},
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "api",
				Datacenter: "dc1",
				Mode:       structs.ProxyModeTransparent,

				// Empty Upstreams/UpstreamIDs
			},
			expect: structs.ServiceConfigResponse{
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": map[string]interface{}{
								"Mode": "remote",
							},
						},
					},
					{
						Upstream: mysql,
						Config: map[string]interface{}{
							"protocol": "grpc",
							"mesh_gateway": map[string]interface{}{
								"Mode": "remote",
							},
						},
					},
				},
			},
		},
		{
			name: "without upstream args we should return centralized config with tproxy default",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "api",
					UpstreamConfig: &structs.UpstreamConfiguration{
						Defaults: &structs.UpstreamConfig{
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
						},
						Overrides: []*structs.UpstreamConfig{
							{
								Name:     "mysql",
								Protocol: "grpc",
							},
						},
					},

					// TransparentProxy on the config entry but not the config request
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "api",
				Datacenter: "dc1",

				// Empty Upstreams/UpstreamIDs
			},
			expect: structs.ServiceConfigResponse{
				Mode: structs.ProxyModeTransparent,
				TransparentProxy: structs.TransparentProxyConfig{
					OutboundListenerPort: 10101,
					DialedDirectly:       true,
				},
				UpstreamIDConfigs: structs.OpaqueUpstreamConfigs{
					{
						Upstream: wildcard,
						Config: map[string]interface{}{
							"mesh_gateway": map[string]interface{}{
								"Mode": "remote",
							},
						},
					},
					{
						Upstream: mysql,
						Config: map[string]interface{}{
							"protocol": "grpc",
							"mesh_gateway": map[string]interface{}{
								"Mode": "remote",
							},
						},
					},
				},
			},
		},
		{
			name: "without upstream args we should NOT return centralized config outside tproxy mode",
			entries: []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "api",
					UpstreamConfig: &structs.UpstreamConfiguration{
						Defaults: &structs.UpstreamConfig{
							MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
						},
						Overrides: []*structs.UpstreamConfig{
							{
								Name:     "mysql",
								Protocol: "grpc",
							},
						},
					},
				},
			},
			request: structs.ServiceConfigRequest{
				Name:       "api",
				Datacenter: "dc1",
				Mode:       structs.ProxyModeDirect,

				// Empty Upstreams/UpstreamIDs
			},
			expect: structs.ServiceConfigResponse{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dir1, s1 := testServer(t)
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()

			codec := rpcClient(t, s1)
			defer codec.Close()

			state := s1.fsm.State()

			// Boostrap the config entries
			idx := uint64(1)
			for _, conf := range tc.entries {
				require.NoError(t, state.EnsureConfigEntry(idx, conf))
				idx++
			}

			var out structs.ServiceConfigResponse
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &tc.request, &out))

			// Don't know what this is deterministically, so we grab it from the response
			tc.expect.QueryMeta = out.QueryMeta

			// Order of this slice is also not deterministic since it's populated from a map
			sort.SliceStable(out.UpstreamIDConfigs, func(i, j int) bool {
				return out.UpstreamIDConfigs[i].Upstream.String() < out.UpstreamIDConfigs[j].Upstream.String()
			})

			require.Equal(t, tc.expect, out)
		})
	}
}

func TestConfigEntry_ResolveServiceConfig_Blocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// The main thing this should test is that information from one iteration
	// of the blocking query does NOT bleed over into the next run. Concretely
	// in this test the data present in the initial proxy-defaults should not
	// be present when we are woken up due to proxy-defaults being deleted.
	//
	// This test does not pertain to upstreams, see:
	// TestConfigEntry_ResolveServiceConfig_Upstreams_Blocking

	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"global": 1,
		},
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "foo",
		Protocol: "grpc",
	}))
	require.NoError(t, state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "bar",
		Protocol: "http",
	}))

	var index uint64

	{ // Verify that we get the results of proxy-defaults and service-defaults for 'foo'.
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
			},
			&out,
		))

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"global":   int64(1),
				"protocol": "grpc",
			},
			QueryMeta: out.QueryMeta,
		}
		require.Equal(t, expected, out)
		index = out.Index
	}

	// Now setup a blocking query for 'foo' while we erase the service-defaults for foo.
	{
		// Async cause a change
		start := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			require.NoError(t, state.DeleteConfigEntry(index+1,
				structs.ServiceDefaults,
				"foo",
				nil,
			))
		}()

		// Re-run the query
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
				QueryOptions: structs.QueryOptions{
					MinQueryIndex: index,
					MaxQueryTime:  time.Second,
				},
			},
			&out,
		))

		// Should block at least 100ms
		require.True(t, time.Since(start) >= 100*time.Millisecond, "too fast")

		// Check the indexes
		require.Equal(t, out.Index, index+1)

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"global": int64(1),
			},
			QueryMeta: out.QueryMeta,
		}
		require.Equal(t, expected, out)

		index = out.Index
	}

	{ // Verify that we get the results of proxy-defaults and service-defaults for 'bar'.
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "bar",
				Datacenter: "dc1",
			},
			&out,
		))

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"global":   int64(1),
				"protocol": "http",
			},
			QueryMeta: out.QueryMeta,
		}
		require.Equal(t, expected, out)
		index = out.Index
	}

	// Now setup a blocking query for 'bar' while we erase the global proxy-defaults.
	{
		// Async cause a change
		start := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			require.NoError(t, state.DeleteConfigEntry(index+1,
				structs.ProxyDefaults,
				structs.ProxyConfigGlobal,
				nil,
			))
		}()

		// Re-run the query
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "bar",
				Datacenter: "dc1",
				QueryOptions: structs.QueryOptions{
					MinQueryIndex: index,
					MaxQueryTime:  time.Second,
				},
			},
			&out,
		))

		// Should block at least 100ms
		require.True(t, time.Since(start) >= 100*time.Millisecond, "too fast")

		// Check the indexes
		require.Equal(t, out.Index, index+1)

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"protocol": "http",
			},
			QueryMeta: out.QueryMeta,
		}
		require.Equal(t, expected, out)
	}
}

func TestConfigEntry_ResolveServiceConfig_Upstreams_Blocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// The main thing this should test is that information from one iteration
	// of the blocking query does NOT bleed over into the next run. Concretely
	// in this test the data present in the initial proxy-defaults should not
	// be present when we are woken up due to proxy-defaults being deleted.
	//
	// This test is about fields in upstreams, see:
	// TestConfigEntry_ResolveServiceConfig_Blocking

	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "foo",
		Protocol: "http",
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "bar",
		Protocol: "http",
	}))

	var index uint64

	testutil.RunStep(t, "foo and bar should be both http", func(t *testing.T) {
		// Verify that we get the results of service-defaults for 'foo' and 'bar'.
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
				UpstreamIDs: []structs.ServiceID{
					structs.NewServiceID("bar", nil),
					structs.NewServiceID("other", nil),
				},
			},
			&out,
		))

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"protocol": "http",
			},
			UpstreamIDConfigs: []structs.OpaqueUpstreamConfig{
				{
					Upstream: structs.NewServiceID("bar", nil),
					Config: map[string]interface{}{
						"protocol": "http",
					},
				},
			},
			QueryMeta: out.QueryMeta, // don't care
		}

		require.Equal(t, expected, out)
		index = out.Index
	})

	testutil.RunStep(t, "blocking query for foo wakes on bar entry delete", func(t *testing.T) {
		// Now setup a blocking query for 'foo' while we erase the
		// service-defaults for bar.

		// Async cause a change
		start := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			err := state.DeleteConfigEntry(index+1,
				structs.ServiceDefaults,
				"bar",
				nil,
			)
			if err != nil {
				t.Errorf("delete config entry failed: %v", err)
			}
		}()

		// Re-run the query
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
				UpstreamIDs: []structs.ServiceID{
					structs.NewServiceID("bar", nil),
					structs.NewServiceID("other", nil),
				},
				QueryOptions: structs.QueryOptions{
					MinQueryIndex: index,
					MaxQueryTime:  time.Second,
				},
			},
			&out,
		))

		// Should block at least 100ms
		require.True(t, time.Since(start) >= 100*time.Millisecond, "too fast")

		// Check the indexes
		require.Equal(t, out.Index, index+1)

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"protocol": "http",
			},
			QueryMeta: out.QueryMeta, // don't care
		}

		require.Equal(t, expected, out)
		index = out.Index
	})

	testutil.RunStep(t, "foo should be http and bar should be unset", func(t *testing.T) {
		// Verify that we get the results of service-defaults for just 'foo'.
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
				UpstreamIDs: []structs.ServiceID{
					structs.NewServiceID("bar", nil),
					structs.NewServiceID("other", nil),
				},
			},
			&out,
		))

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"protocol": "http",
			},
			QueryMeta: out.QueryMeta, // don't care
		}

		require.Equal(t, expected, out)
		index = out.Index
	})

	testutil.RunStep(t, "blocking query for foo wakes on foo entry delete", func(t *testing.T) {
		// Now setup a blocking query for 'foo' while we erase the
		// service-defaults for foo.

		// Async cause a change
		start := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			err := state.DeleteConfigEntry(index+1,
				structs.ServiceDefaults,
				"foo",
				nil,
			)
			if err != nil {
				t.Errorf("delete config entry failed: %v", err)
			}
		}()

		// Re-run the query
		var out structs.ServiceConfigResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
			&structs.ServiceConfigRequest{
				Name:       "foo",
				Datacenter: "dc1",
				UpstreamIDs: []structs.ServiceID{
					structs.NewServiceID("bar", nil),
					structs.NewServiceID("other", nil),
				},
				QueryOptions: structs.QueryOptions{
					MinQueryIndex: index,
					MaxQueryTime:  time.Second,
				},
			},
			&out,
		))

		// Should block at least 100ms
		require.True(t, time.Since(start) >= 100*time.Millisecond, "too fast")

		// Check the indexes
		require.Equal(t, out.Index, index+1)

		expected := structs.ServiceConfigResponse{
			QueryMeta: out.QueryMeta, // don't care
		}

		require.Equal(t, expected, out)
		index = out.Index
	})
}

func TestConfigEntry_ResolveServiceConfig_UpstreamProxyDefaultsProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create a dummy proxy/service config in the state store to look up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "bar",
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "other",
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "alreadyprotocol",
		Protocol: "grpc",
	}))

	args := structs.ServiceConfigRequest{
		Name:       "foo",
		Datacenter: s1.config.Datacenter,
		Upstreams:  []string{"bar", "other", "alreadyprotocol", "dne"},
	}
	var out structs.ServiceConfigResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

	expected := structs.ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"protocol": "http",
		},
		UpstreamConfigs: map[string]map[string]interface{}{
			"bar": {
				"protocol": "http",
			},
			"other": {
				"protocol": "http",
			},
			"dne": {
				"protocol": "http",
			},
			"alreadyprotocol": {
				"protocol": "grpc",
			},
		},
		// Don't know what this is deterministically
		QueryMeta: out.QueryMeta,
	}
	require.Equal(t, expected, out)
}

func TestConfigEntry_ResolveServiceConfig_ProxyDefaultsProtocol_UsedForAllUpstreams(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create a dummy proxy/service config in the state store to look up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))

	args := structs.ServiceConfigRequest{
		Name:       "foo",
		Datacenter: s1.config.Datacenter,
		Upstreams:  []string{"bar"},
	}
	var out structs.ServiceConfigResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

	expected := structs.ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"protocol": "http",
		},
		UpstreamConfigs: map[string]map[string]interface{}{
			"bar": {
				"protocol": "http",
			},
		},
		// Don't know what this is deterministically
		QueryMeta: out.QueryMeta,
	}
	require.Equal(t, expected, out)
}

func BenchmarkConfigEntry_ResolveServiceConfig_Hash(b *testing.B) {
	res := &configentry.ResolvedServiceConfigSet{}

	res.AddServiceDefaults(&structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "web",
		Protocol: "http",
	})
	res.AddServiceDefaults(&structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "up1",
		Protocol: "http",
	})
	res.AddServiceDefaults(&structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "up2",
		Protocol: "http",
	})
	res.AddProxyDefaults(&structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "grpc",
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hashstructure_v2.Hash(res, hashstructure_v2.FormatV2, nil)
		if err != nil {
			b.Fatalf("error: %v", err)
		}
	}
}

func TestConfigEntry_ResolveServiceConfig_BlockOnNoChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
	})

	codec := rpcClient(t, s1)

	run := func(t *testing.T, dataPrefix string) {
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := structs.ServiceConfigRequest{
					Name: "foo",
					UpstreamIDs: []structs.ServiceID{
						structs.NewServiceID("bar", nil),
					},
				}
				args.QueryOptions.MinQueryIndex = minQueryIndex

				var out structs.ServiceConfigResponse

				errCh := channelCallRPC(s1, "ConfigEntry.ResolveServiceConfig", &args, &out, nil)
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out bool
				return channelCallRPC(s1, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
					Entry: &structs.ServiceConfigEntry{
						Kind: structs.ServiceDefaults,
						Name: fmt.Sprintf(dataPrefix+"%d", i),
					},
				}, &out, func() error {
					if !out {
						return fmt.Errorf("[%d] unexpectedly returned false", i)
					}
					return nil
				})
			},
		)
	}

	{ // create one unrelated entry
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Entry: &structs.ServiceConfigEntry{
				Kind: structs.ServiceDefaults,
				Name: "unrelated",
			},
		}, &out))
		require.True(t, out)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
		run(t, "other")
	})

	{ // create one relevant entry
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "bar",
				Protocol: "grpc",
			},
		}, &out))
		require.True(t, out)
	}

	testutil.RunStep(t, "test the errNotChanged path", func(t *testing.T) {
		run(t, "completely-different-other")
	})
}

func TestConfigEntry_ResolveServiceConfigNoConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Don't create any config and make sure we don't nil panic (spoiler alert -
	// we did in first RC)
	args := structs.ServiceConfigRequest{
		Name:       "foo",
		Datacenter: s1.config.Datacenter,
		Upstreams:  []string{"bar", "baz"},
	}
	var out structs.ServiceConfigResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

	expected := structs.ServiceConfigResponse{
		ProxyConfig:     nil,
		UpstreamConfigs: nil,
		// Don't know what this is deterministically
		QueryMeta: out.QueryMeta,
	}
	require.Equal(t, expected, out)
}

func TestConfigEntry_ResolveServiceConfig_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
service "foo" {
	policy = "write"
}
operator = "write"
`
	id := createToken(t, codec, rules)

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(t, state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(t, state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "db",
	}))

	// This should fail since we don't have write perms for the "db" service.
	args := structs.ServiceConfigRequest{
		Name:         "db",
		Datacenter:   s1.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var out structs.ServiceConfigResponse
	err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The "foo" service should work.
	args.Name = "foo"
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

}

func TestConfigEntry_ProxyDefaultsExposeConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	expose := structs.ExposeConfig{
		Checks: true,
		Paths: []structs.ExposePath{
			{
				LocalPathPort: 8080,
				ListenerPort:  21500,
				Protocol:      "http2",
				Path:          "/healthz",
			},
		},
	}

	args := structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Entry: &structs.ProxyConfigEntry{
			Kind:   "proxy-defaults",
			Name:   "global",
			Expose: expose,
		},
	}

	out := false
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
	require.True(t, out)

	state := s1.fsm.State()
	_, entry, err := state.ConfigEntry(nil, structs.ProxyDefaults, "global", nil)
	require.NoError(t, err)

	proxyConf, ok := entry.(*structs.ProxyConfigEntry)
	require.True(t, ok)
	require.Equal(t, expose, proxyConf.Expose)
}

func Test_gateWriteToSecondary(t *testing.T) {
	type args struct {
		targetDC  string
		localDC   string
		primaryDC string
		kind      string
	}
	type testCase struct {
		name    string
		args    args
		wantErr string
	}

	run := func(t *testing.T, tc testCase) {
		err := gateWriteToSecondary(tc.args.targetDC, tc.args.localDC, tc.args.primaryDC, tc.args.kind)
		if tc.wantErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
			return
		}
		require.NoError(t, err)
	}

	tt := []testCase{
		{
			name: "primary to primary with implicit primary and target",
			args: args{
				targetDC:  "",
				localDC:   "dc1",
				primaryDC: "",
				kind:      structs.ExportedServices,
			},
		},
		{
			name: "primary to primary with explicit primary and implicit target",
			args: args{
				targetDC:  "",
				localDC:   "dc1",
				primaryDC: "dc1",
				kind:      structs.ExportedServices,
			},
		},
		{
			name: "primary to primary with all filled in",
			args: args{
				targetDC:  "dc1",
				localDC:   "dc1",
				primaryDC: "dc1",
				kind:      structs.ExportedServices,
			},
		},
		{
			name: "primary to secondary with implicit primary and target",
			args: args{
				targetDC:  "dc2",
				localDC:   "dc1",
				primaryDC: "",
				kind:      structs.ExportedServices,
			},
			wantErr: "writes must not target secondary datacenters",
		},
		{
			name: "primary to secondary with all filled in",
			args: args{
				targetDC:  "dc2",
				localDC:   "dc1",
				primaryDC: "dc1",
				kind:      structs.ExportedServices,
			},
			wantErr: "writes must not target secondary datacenters",
		},
		{
			name: "secondary to secondary with all filled in",
			args: args{
				targetDC:  "dc2",
				localDC:   "dc2",
				primaryDC: "dc1",
				kind:      structs.ExportedServices,
			},
			wantErr: "writes must not target secondary datacenters",
		},
		{
			name: "implicit write to secondary",
			args: args{
				targetDC:  "",
				localDC:   "dc2",
				primaryDC: "dc1",
				kind:      structs.ExportedServices,
			},
			wantErr: "must target the primary datacenter explicitly",
		},
		{
			name: "empty local DC",
			args: args{
				localDC: "",
				kind:    structs.ExportedServices,
			},
			wantErr: "unknown local datacenter",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func Test_gateWriteToSecondary_AllowedKinds(t *testing.T) {
	type args struct {
		targetDC  string
		localDC   string
		primaryDC string
		kind      string
	}

	for _, kind := range structs.AllConfigEntryKinds {
		if kind == structs.ExportedServices {
			continue
		}

		t.Run(fmt.Sprintf("%s-secondary-to-secondary", kind), func(t *testing.T) {
			tcase := args{
				targetDC:  "",
				localDC:   "dc2",
				primaryDC: "dc1",
				kind:      kind,
			}
			require.NoError(t, gateWriteToSecondary(tcase.targetDC, tcase.localDC, tcase.primaryDC, tcase.kind))
		})

		t.Run(fmt.Sprintf("%s-primary-to-secondary", kind), func(t *testing.T) {
			tcase := args{
				targetDC:  "dc2",
				localDC:   "dc1",
				primaryDC: "dc1",
				kind:      kind,
			}
			require.NoError(t, gateWriteToSecondary(tcase.targetDC, tcase.localDC, tcase.primaryDC, tcase.kind))
		})
	}
}

func uintPointer(v uint32) *uint32 {
	return &v
}
