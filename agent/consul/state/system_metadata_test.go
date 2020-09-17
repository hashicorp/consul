package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestStore_SystemMetadata(t *testing.T) {
	s := testStateStore(t)

	mapify := func(entries []*structs.SystemMetadataEntry) map[string]string {
		m := make(map[string]string)
		for _, entry := range entries {
			m[entry.Key] = entry.Value
		}
		return m
	}

	checkListAndGet := func(t *testing.T, expect map[string]string) {
		// List all
		_, entries, err := s.SystemMetadataList(nil)
		require.NoError(t, err)
		require.Len(t, entries, len(expect))
		require.Equal(t, expect, mapify(entries))

		// Read each
		for expectKey, expectVal := range expect {
			_, entry, err := s.SystemMetadataGet(nil, expectKey)
			require.NoError(t, err)
			require.NotNil(t, entry)
			require.Equal(t, expectVal, entry.Value)
		}
	}

	checkListAndGet(t, map[string]string{})

	// Create 3 keys
	require.NoError(t, s.SystemMetadataSet(1, []*structs.SystemMetadataEntry{
		{Key: "key1", Value: "val1"},
		{Key: "key2", Value: "val2"},
		{Key: "key3"},
	}))

	checkListAndGet(t, map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "",
	})

	// Missing results are nil
	_, entry, err := s.SystemMetadataGet(nil, "key4")
	require.NoError(t, err)
	require.Nil(t, entry)

	// Delete one that exists and one that does not
	require.NoError(t, s.SystemMetadataDelete(2, []*structs.SystemMetadataEntry{
		{Key: "key2"},
		{Key: "key4"},
	}))

	checkListAndGet(t, map[string]string{
		"key1": "val1",
		"key3": "",
	})

	// Update one that exists and add another one.
	require.NoError(t, s.SystemMetadataSet(1, []*structs.SystemMetadataEntry{
		{Key: "key3", Value: "val3"},
		{Key: "key4", Value: "val4"},
	}))

	checkListAndGet(t, map[string]string{
		"key1": "val1",
		"key3": "val3",
		"key4": "val4",
	})

}
