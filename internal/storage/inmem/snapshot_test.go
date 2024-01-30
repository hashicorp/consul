// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package inmem_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestSnapshotRestore(t *testing.T) {
	oldStore, err := inmem.NewStore()
	require.NoError(t, err)

	a := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "mesh",
				GroupVersion: "v1",
				Kind:         "service",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
			},
			Name: "billing",
			Uid:  "a",
		},
		Version: "1",
	}
	require.NoError(t, oldStore.WriteCAS(a, ""))

	newStore, err := inmem.NewStore()
	require.NoError(t, err)

	// Write something to the new store to make sure it gets blown away.
	b := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "mesh",
				GroupVersion: "v1",
				Kind:         "service",
			},
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
			},
			Name: "api",
			Uid:  "a",
		},
		Version: "1",
	}
	require.NoError(t, newStore.WriteCAS(b, ""))

	snap, err := oldStore.Snapshot()
	require.NoError(t, err)

	// Start a watch on the new store to make sure it gets closed.
	watch, err := newStore.WatchList(storage.UnversionedTypeFrom(b.Id.Type), b.Id.Tenancy, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Expect the initial state on the watch.
	_, err = watch.Next(ctx)
	require.NoError(t, err)

	restore, err := newStore.Restore()
	require.NoError(t, err)
	defer restore.Abort()

	for r := snap.Next(); r != nil; r = snap.Next() {
		restore.Apply(r)
	}
	restore.Commit()

	// Check that resource we wrote to oldStore has been restored to newStore.
	rsp, err := newStore.Read(a.Id)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, a, rsp)

	// Check that resource written to newStore was removed by snapshot restore.
	_, err = newStore.Read(b.Id)
	require.ErrorIs(t, err, storage.ErrNotFound)

	// Check the watch has been closed.
	_, err = watch.Next(ctx)
	require.ErrorIs(t, err, storage.ErrWatchClosed)
}
