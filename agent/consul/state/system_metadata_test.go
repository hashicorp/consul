// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

	var nextIndex uint64

	// Create 3 keys
	nextIndex++
	require.NoError(t, s.SystemMetadataSet(nextIndex, &structs.SystemMetadataEntry{
		Key: "key1", Value: "val1",
	}))
	nextIndex++
	require.NoError(t, s.SystemMetadataSet(nextIndex, &structs.SystemMetadataEntry{
		Key: "key2", Value: "val2",
	}))
	nextIndex++
	require.NoError(t, s.SystemMetadataSet(nextIndex, &structs.SystemMetadataEntry{
		Key: "key3",
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
	nextIndex++
	require.NoError(t, s.SystemMetadataDelete(nextIndex, &structs.SystemMetadataEntry{
		Key: "key2",
	}))
	nextIndex++
	require.NoError(t, s.SystemMetadataDelete(nextIndex, &structs.SystemMetadataEntry{
		Key: "key4",
	}))

	checkListAndGet(t, map[string]string{
		"key1": "val1",
		"key3": "",
	})

	// Update one that exists and add another one.
	nextIndex++
	require.NoError(t, s.SystemMetadataSet(nextIndex, &structs.SystemMetadataEntry{
		Key: "key3", Value: "val3",
	}))
	require.NoError(t, s.SystemMetadataSet(nextIndex, &structs.SystemMetadataEntry{
		Key: "key4", Value: "val4",
	}))

	checkListAndGet(t, map[string]string{
		"key1": "val1",
		"key3": "val3",
		"key4": "val4",
	})

}
