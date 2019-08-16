package consul

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"
)

func TestConfigEntry_Apply(t *testing.T) {
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

	updated := &structs.ServiceConfigEntry{
		Name: "foo",
	}
	// originally target this as going to dc2
	args := structs.ConfigEntryRequest{
		Datacenter: "dc2",
		Entry:      updated,
	}
	out := false
	require.NoError(t, msgpackrpc.CallWithCodec(codec2, "ConfigEntry.Apply", &args, &out))
	require.True(t, out)

	// the previous RPC should not return until the primary has been updated but will return
	// before the secondary has the data.
	state := s1.fsm.State()
	_, entry, err := state.ConfigEntry(nil, structs.ServiceDefaults, "foo")
	require.NoError(t, err)

	serviceConf, ok := entry.(*structs.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, "foo", serviceConf.Name)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)

	retry.Run(t, func(r *retry.R) {
		// wait for replication to happen
		state := s2.fsm.State()
		_, entry, err := state.ConfigEntry(nil, structs.ServiceDefaults, "foo")
		require.NoError(r, err)
		require.NotNil(r, entry)
		// this test is not testing that the config entries that are replicated are correct as thats done elsewhere.
	})

	updated = &structs.ServiceConfigEntry{
		Name: "foo",
		MeshGateway: structs.MeshGatewayConfig{
			Mode: structs.MeshGatewayModeLocal,
		},
	}

	args = structs.ConfigEntryRequest{
		Datacenter: "dc1",
		Op:         structs.ConfigEntryUpsertCAS,
		Entry:      updated,
	}

	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
	require.False(t, out)

	args.Entry.GetRaftIndex().ModifyIndex = serviceConf.ModifyIndex
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
	require.True(t, out)

	state = s1.fsm.State()
	_, entry, err = state.ConfigEntry(nil, structs.ServiceDefaults, "foo")
	require.NoError(t, err)

	serviceConf, ok = entry.(*structs.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)
	require.Equal(t, "foo", serviceConf.Name)
	require.Equal(t, "", serviceConf.Protocol)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)
}

func TestConfigEntry_ProxyDefaultsMeshGateway(t *testing.T) {
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
	_, entry, err := state.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(t, err)

	proxyConf, ok := entry.(*structs.ProxyConfigEntry)
	require.True(t, ok)
	require.Equal(t, structs.MeshGatewayModeLocal, proxyConf.MeshGateway.Mode)
}

func TestConfigEntry_Apply_ACLDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
operator = "write"
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

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
	require.NoError(err)

	state := s1.fsm.State()
	_, entry, err := state.ConfigEntry(nil, structs.ServiceDefaults, "foo")
	require.NoError(err)

	serviceConf, ok := entry.(*structs.ServiceConfigEntry)
	require.True(ok)
	require.Equal("foo", serviceConf.Name)
	require.Equal(structs.ServiceDefaults, serviceConf.Kind)

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
	require.NoError(err)
}

func TestConfigEntry_Get(t *testing.T) {
	t.Parallel()

	require := require.New(t)

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
	require.NoError(state.EnsureConfigEntry(1, entry))

	args := structs.ConfigEntryQuery{
		Kind:       structs.ServiceDefaults,
		Name:       "foo",
		Datacenter: s1.config.Datacenter,
	}
	var out structs.ConfigEntryResponse
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.Get", &args, &out))

	serviceConf, ok := out.Entry.(*structs.ServiceConfigEntry)
	require.True(ok)
	require.Equal("foo", serviceConf.Name)
	require.Equal(structs.ServiceDefaults, serviceConf.Kind)
}

func TestConfigEntry_Get_ACLDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "read"
}
operator = "read"
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
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
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.Get", &args, &out))

	serviceConf, ok := out.Entry.(*structs.ServiceConfigEntry)
	require.True(ok)
	require.Equal("foo", serviceConf.Name)
	require.Equal(structs.ServiceDefaults, serviceConf.Kind)
}

func TestConfigEntry_List(t *testing.T) {
	t.Parallel()

	require := require.New(t)

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
	require.NoError(state.EnsureConfigEntry(1, expected.Entries[0]))
	require.NoError(state.EnsureConfigEntry(2, expected.Entries[1]))

	args := structs.ConfigEntryQuery{
		Kind:       structs.ServiceDefaults,
		Datacenter: "dc1",
	}
	var out structs.IndexedConfigEntries
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.List", &args, &out))

	expected.Kind = structs.ServiceDefaults
	expected.QueryMeta = out.QueryMeta
	require.Equal(expected, out)
}

func TestConfigEntry_ListAll(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create some dummy services in the state store to look up.
	state := s1.fsm.State()
	expected := structs.IndexedGenericConfigEntries{
		Entries: []structs.ConfigEntry{
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
		},
	}
	require.NoError(state.EnsureConfigEntry(1, expected.Entries[0]))
	require.NoError(state.EnsureConfigEntry(2, expected.Entries[1]))
	require.NoError(state.EnsureConfigEntry(3, expected.Entries[2]))

	args := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedGenericConfigEntries
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ListAll", &args, &out))

	expected.QueryMeta = out.QueryMeta
	require.Equal(expected, out)
}

func TestConfigEntry_List_ACLDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "read"
}
operator = "read"
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
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
	require.NoError(err)

	serviceConf, ok := out.Entries[0].(*structs.ServiceConfigEntry)
	require.Len(out.Entries, 1)
	require.True(ok)
	require.Equal("foo", serviceConf.Name)
	require.Equal(structs.ServiceDefaults, serviceConf.Kind)

	// Get the global proxy config.
	args.Kind = structs.ProxyDefaults
	err = msgpackrpc.CallWithCodec(codec, "ConfigEntry.List", &args, &out)
	require.NoError(err)

	proxyConf, ok := out.Entries[0].(*structs.ProxyConfigEntry)
	require.Len(out.Entries, 1)
	require.True(ok)
	require.Equal(structs.ProxyConfigGlobal, proxyConf.Name)
	require.Equal(structs.ProxyDefaults, proxyConf.Kind)
}

func TestConfigEntry_ListAll_ACLDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "read"
}
operator = "read"
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "db",
	}))

	// This should filter out the "db" service since we don't have permissions for it.
	args := structs.ConfigEntryQuery{
		Datacenter:   s1.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var out structs.IndexedGenericConfigEntries
	err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.ListAll", &args, &out)
	require.NoError(err)
	require.Len(out.Entries, 2)
	svcIndex := 0
	proxyIndex := 1
	if out.Entries[0].GetKind() == structs.ProxyDefaults {
		svcIndex = 1
		proxyIndex = 0
	}

	svcConf, ok := out.Entries[svcIndex].(*structs.ServiceConfigEntry)
	require.True(ok)
	proxyConf, ok := out.Entries[proxyIndex].(*structs.ProxyConfigEntry)
	require.True(ok)

	require.Equal("foo", svcConf.Name)
	require.Equal(structs.ServiceDefaults, svcConf.Kind)
	require.Equal(structs.ProxyConfigGlobal, proxyConf.Name)
	require.Equal(structs.ProxyDefaults, proxyConf.Kind)
}

func TestConfigEntry_Delete(t *testing.T) {
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

	// Create a dummy service in the state store to look up.
	entry := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}
	state := s1.fsm.State()
	require.NoError(t, state.EnsureConfigEntry(1, entry))

	// Verify it's there.
	_, existing, err := state.ConfigEntry(nil, structs.ServiceDefaults, "foo")
	require.NoError(t, err)

	serviceConf, ok := existing.(*structs.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, "foo", serviceConf.Name)
	require.Equal(t, structs.ServiceDefaults, serviceConf.Kind)

	retry.Run(t, func(r *retry.R) {
		// wait for it to be replicated into the secondary dc
		_, existing, err := s2.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo")
		require.NoError(r, err)
		require.NotNil(r, existing)
	})

	// send the delete request to dc2 - it should get forwarded to dc1.
	args := structs.ConfigEntryRequest{
		Datacenter: "dc2",
	}
	args.Entry = entry
	var out struct{}
	require.NoError(t, msgpackrpc.CallWithCodec(codec2, "ConfigEntry.Delete", &args, &out))

	// Verify the entry was deleted.
	_, existing, err = s1.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo")
	require.NoError(t, err)
	require.Nil(t, existing)

	// verify it gets deleted from the secondary too
	retry.Run(t, func(r *retry.R) {
		_, existing, err := s2.fsm.State().ConfigEntry(nil, structs.ServiceDefaults, "foo")
		require.NoError(r, err)
		require.Nil(r, existing)
	})
}

func TestConfigEntry_Delete_ACLDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
operator = "write"
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
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
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &out))

	// Verify the entry was deleted.
	_, existing, err := state.ConfigEntry(nil, structs.ServiceDefaults, "foo")
	require.NoError(err)
	require.Nil(existing)

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
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.Delete", &args, &out))

	_, existing, err = state.ConfigEntry(nil, structs.ServiceDefaults, "foo")
	require.NoError(err)
	require.Nil(existing)
}

func TestConfigEntry_ResolveServiceConfig(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create a dummy proxy/service config in the state store to look up.
	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"foo": 1,
		},
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "foo",
		Protocol: "http",
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
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
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

	expected := structs.ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"foo":      int64(1),
			"protocol": "http",
		},
		UpstreamConfigs: map[string]map[string]interface{}{
			"bar": map[string]interface{}{
				"protocol": "grpc",
			},
		},
		// Don't know what this is deterministically
		QueryMeta: out.QueryMeta,
	}
	require.Equal(expected, out)

	_, entry, err := s1.fsm.State().ConfigEntry(nil, structs.ProxyDefaults, structs.ProxyConfigGlobal)
	require.NoError(err)
	require.NotNil(entry)
	proxyConf, ok := entry.(*structs.ProxyConfigEntry)
	require.True(ok)
	require.Equal(map[string]interface{}{"foo": 1}, proxyConf.Config)
}

func TestConfigEntry_ResolveServiceConfig_Blocking(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// The main thing this should test is that information from one iteration
	// of the blocking query does NOT bleed over into the next run. Concretely
	// in this test the data present in the initial proxy-defaults should not
	// be present when we are woken up due to proxy-defaults being deleted.

	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"global": 1,
		},
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "foo",
		Protocol: "grpc",
	}))
	require.NoError(state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "bar",
		Protocol: "http",
	}))

	var index uint64

	{ // Verify that we get the results of proxy-defaults and service-defaults for 'foo'.
		var out structs.ServiceConfigResponse
		require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
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
		require.Equal(expected, out)
		index = out.Index
	}

	// Now setup a blocking query for 'foo' while we erase the service-defaults for foo.
	{
		// Async cause a change
		start := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			require.NoError(state.DeleteConfigEntry(index+1,
				structs.ServiceDefaults,
				"foo",
			))
		}()

		// Re-run the query
		var out structs.ServiceConfigResponse
		require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
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
		require.True(time.Since(start) >= 100*time.Millisecond, "too fast")

		// Check the indexes
		require.Equal(out.Index, index+1)

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"global": int64(1),
			},
			QueryMeta: out.QueryMeta,
		}
		require.Equal(expected, out)

		index = out.Index
	}

	{ // Verify that we get the results of proxy-defaults and service-defaults for 'bar'.
		var out structs.ServiceConfigResponse
		require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
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
		require.Equal(expected, out)
		index = out.Index
	}

	// Now setup a blocking query for 'bar' while we erase the global proxy-defaults.
	{
		// Async cause a change
		start := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			require.NoError(state.DeleteConfigEntry(index+1,
				structs.ProxyDefaults,
				structs.ProxyConfigGlobal,
			))
		}()

		// Re-run the query
		var out structs.ServiceConfigResponse
		require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig",
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
		require.True(time.Since(start) >= 100*time.Millisecond, "too fast")

		// Check the indexes
		require.Equal(out.Index, index+1)

		expected := structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{
				"protocol": "http",
			},
			QueryMeta: out.QueryMeta,
		}
		require.Equal(expected, out)
	}
}

func TestConfigEntry_ResolveServiceConfig_UpstreamProxyDefaultsProtocol(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create a dummy proxy/service config in the state store to look up.
	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "bar",
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "other",
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
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
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

	expected := structs.ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"protocol": "http",
		},
		UpstreamConfigs: map[string]map[string]interface{}{
			"bar": map[string]interface{}{
				"protocol": "http",
			},
			"other": map[string]interface{}{
				"protocol": "http",
			},
			"alreadyprotocol": map[string]interface{}{
				"protocol": "grpc",
			},
		},
		// Don't know what this is deterministically
		QueryMeta: out.QueryMeta,
	}
	require.Equal(expected, out)
}

func TestConfigEntry_ResolveServiceConfigNoConfig(t *testing.T) {
	t.Parallel()

	require := require.New(t)

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
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

	expected := structs.ServiceConfigResponse{
		ProxyConfig:     nil,
		UpstreamConfigs: nil,
		// Don't know what this is deterministically
		QueryMeta: out.QueryMeta,
	}
	require.Equal(expected, out)
}

func TestConfigEntry_ResolveServiceConfig_ACLDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create the ACL.
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name: "User token",
			Type: structs.ACLTokenTypeClient,
			Rules: `
service "foo" {
	policy = "write"
}
operator = "write"
`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some dummy service/proxy configs to be looked up.
	state := s1.fsm.State()
	require.NoError(state.EnsureConfigEntry(1, &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
	}))
	require.NoError(state.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}))
	require.NoError(state.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
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
	require.NoError(msgpackrpc.CallWithCodec(codec, "ConfigEntry.ResolveServiceConfig", &args, &out))

}
