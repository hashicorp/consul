package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntries_ExportedServices(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	entries := c.ConfigEntries()

	runStep(t, "set and get", func(t *testing.T) {
		exports := &ExportedServicesConfigEntry{
			Name:      PartitionDefaultName,
			Partition: defaultPartition,
			Meta: map[string]string{
				"gir": "zim",
			},
		}

		_, wm, err := entries.Set(exports, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		entry, qm, err := entries.Get(ExportedServices, PartitionDefaultName, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		result, ok := entry.(*ExportedServicesConfigEntry)
		require.True(t, ok)

		// ignore indexes
		result.CreateIndex = 0
		result.ModifyIndex = 0
		require.Equal(t, exports, result)
	})

	runStep(t, "update", func(t *testing.T) {
		updated := &ExportedServicesConfigEntry{
			Name: PartitionDefaultName,
			Services: []ExportedService{
				{
					Name:      "db",
					Namespace: defaultNamespace,
					Consumers: []ServiceConsumer{
						{
							PeerName: "alpha",
						},
					},
				},
			},
			Meta: map[string]string{
				"foo": "bar",
				"gir": "zim",
			},
			Partition: defaultPartition,
		}

		_, wm, err := entries.Set(updated, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		entry, qm, err := entries.Get(ExportedServices, PartitionDefaultName, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		result, ok := entry.(*ExportedServicesConfigEntry)
		require.True(t, ok)

		// ignore indexes
		result.CreateIndex = 0
		result.ModifyIndex = 0
		require.Equal(t, updated, result)
	})

	runStep(t, "list", func(t *testing.T) {
		entries, qm, err := entries.List(ExportedServices, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)
		require.Len(t, entries, 1)
	})

	runStep(t, "delete", func(t *testing.T) {
		wm, err := entries.Delete(ExportedServices, PartitionDefaultName, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// verify deletion
		_, _, err = entries.Get(MeshConfig, PartitionDefaultName, nil)
		require.Error(t, err)
	})
}
