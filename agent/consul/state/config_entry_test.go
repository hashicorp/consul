package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

func TestStore_ConfigEntry(t *testing.T) {
	require := require.New(t)
	s := testStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(s.EnsureConfigEntry(0, expected))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(0), idx)
	require.Equal(expected, config)

	// Update
	updated := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "bar",
		},
	}
	require.NoError(s.EnsureConfigEntry(1, updated))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(updated, config)

	// Delete
	require.NoError(s.DeleteConfigEntry(2, structs.ProxyDefaults, "global"))

	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Nil(config)

	// Set up a watch.
	serviceConf := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo",
	}
	require.NoError(s.EnsureConfigEntry(3, serviceConf))

	ws := memdb.NewWatchSet()
	_, _, err = s.ConfigEntry(ws, structs.ServiceDefaults, "foo")
	require.NoError(err)

	// Make an unrelated modification and make sure the watch doesn't fire.
	require.NoError(s.EnsureConfigEntry(4, updated))
	require.False(watchFired(ws))

	// Update the watched config and make sure it fires.
	serviceConf.Protocol = "http"
	require.NoError(s.EnsureConfigEntry(5, serviceConf))
	require.True(watchFired(ws))
}

func TestStore_ConfigEntryCAS(t *testing.T) {
	require := require.New(t)
	s := testStateStore(t)

	expected := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "foo",
		},
	}

	// Create
	require.NoError(s.EnsureConfigEntry(1, expected))

	idx, config, err := s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(expected, config)

	// Update with invalid index
	updated := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "global",
		Config: map[string]interface{}{
			"DestinationServiceName": "bar",
		},
	}
	ok, err := s.EnsureConfigEntryCAS(2, 99, updated)
	require.False(ok)
	require.NoError(err)

	// Entry should not be changed
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(1), idx)
	require.Equal(expected, config)

	// Update with a valid index
	ok, err = s.EnsureConfigEntryCAS(2, 1, updated)
	require.True(ok)
	require.NoError(err)

	// Entry should be updated
	idx, config, err = s.ConfigEntry(nil, structs.ProxyDefaults, "global")
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal(updated, config)
}

func TestStore_ConfigEntries(t *testing.T) {
	require := require.New(t)
	s := testStateStore(t)

	// Create some config entries.
	entry1 := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: "test1",
	}
	entry2 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "test2",
	}
	entry3 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "test3",
	}

	require.NoError(s.EnsureConfigEntry(0, entry1))
	require.NoError(s.EnsureConfigEntry(1, entry2))
	require.NoError(s.EnsureConfigEntry(2, entry3))

	// Get all entries
	idx, entries, err := s.ConfigEntries(nil)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry1, entry2, entry3}, entries)

	// Get all proxy entries
	idx, entries, err = s.ConfigEntriesByKind(nil, structs.ProxyDefaults)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry1}, entries)

	// Get all service entries
	ws := memdb.NewWatchSet()
	idx, entries, err = s.ConfigEntriesByKind(ws, structs.ServiceDefaults)
	require.NoError(err)
	require.Equal(uint64(2), idx)
	require.Equal([]structs.ConfigEntry{entry2, entry3}, entries)

	// Watch should not have fired
	require.False(watchFired(ws))

	// Now make an update and make sure the watch fires.
	require.NoError(s.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "test2",
		Protocol: "tcp",
	}))
	require.True(watchFired(ws))
}
