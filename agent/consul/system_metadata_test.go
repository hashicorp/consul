// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestLeader_SystemMetadata_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This test is a little strange because it is testing behavior that
	// doesn't have an exposed RPC. We're just testing the full round trip of
	// raft+fsm For now,

	dir1, srv := testServerWithConfig(t, func(c *Config) {
		// We disable connect here so we skip inserting intention-migration
		// related system metadata in the background.
		c.ConnectEnabled = false
	})
	defer os.RemoveAll(dir1)
	defer srv.Shutdown()
	codec := rpcClient(t, srv)
	defer codec.Close()

	testrpc.WaitForLeader(t, srv.RPC, "dc1")

	state := srv.fsm.State()

	// Initially has no entries
	_, entries, err := state.SystemMetadataList(nil)
	require.NoError(t, err)
	require.Len(t, entries, 0)

	// Create 3
	require.NoError(t, srv.SetSystemMetadataKey("key1", "val1"))
	require.NoError(t, srv.SetSystemMetadataKey("key2", "val2"))
	require.NoError(t, srv.SetSystemMetadataKey("key3", ""))

	mapify := func(entries []*structs.SystemMetadataEntry) map[string]string {
		m := make(map[string]string)
		for _, entry := range entries {
			m[entry.Key] = entry.Value
		}
		return m
	}

	_, entries, err = state.SystemMetadataList(nil)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	require.Equal(t, map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "",
	}, mapify(entries))

	// Update one and delete one.
	require.NoError(t, srv.SetSystemMetadataKey("key3", "val3"))
	require.NoError(t, srv.deleteSystemMetadataKey("key1"))

	_, entries, err = state.SystemMetadataList(nil)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	require.Equal(t, map[string]string{
		"key2": "val2",
		"key3": "val3",
	}, mapify(entries))
}
