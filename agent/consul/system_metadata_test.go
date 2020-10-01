package consul

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestLeader_SystemMetadata_CRUD(t *testing.T) {
	// This test is a little strange because it is testing behavior that
	// doesn't have an exposed RPC. We're just testing the full round trip of
	// raft+fsm For now,

	dir1, srv := testServerWithConfig(t, nil)
	defer os.RemoveAll(dir1)
	defer srv.Shutdown()
	codec := rpcClient(t, srv)
	defer codec.Close()

	testrpc.WaitForLeader(t, srv.RPC, "dc1")

	state := srv.fsm.State()

	// Initially empty
	_, entries, err := state.SystemMetadataList(nil)
	require.NoError(t, err)
	require.Len(t, entries, 0)

	// Create 3
	require.NoError(t, setSystemMetadataKey(srv, "key1", "val1"))
	require.NoError(t, setSystemMetadataKey(srv, "key2", "val2"))
	require.NoError(t, setSystemMetadataKey(srv, "key3", ""))

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
	require.NoError(t, setSystemMetadataKey(srv, "key3", "val3"))
	require.NoError(t, deleteSystemMetadataKey(srv, "key1"))

	_, entries, err = state.SystemMetadataList(nil)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	require.Equal(t, map[string]string{
		"key2": "val2",
		"key3": "val3",
	}, mapify(entries))
}

// Note when this behavior is actually used, consider promoting these 2
// functions out of test code.

func setSystemMetadataKey(s *Server, key, val string) error {
	args := &structs.SystemMetadataRequest{
		Op: structs.SystemMetadataUpsert,
		Entries: []*structs.SystemMetadataEntry{
			{Key: key, Value: val},
		},
	}

	resp, err := s.raftApply(structs.SystemMetadataRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}

func deleteSystemMetadataKey(s *Server, key string) error {
	args := &structs.SystemMetadataRequest{
		Op: structs.SystemMetadataDelete,
		Entries: []*structs.SystemMetadataEntry{
			{Key: key},
		},
	}

	resp, err := s.raftApply(structs.SystemMetadataRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}
