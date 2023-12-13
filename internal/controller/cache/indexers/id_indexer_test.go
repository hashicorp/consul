// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers/indexersmock"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestIDIndex(t *testing.T) {
	idx := IDIndex("test", index.IndexRequired)

	r1 := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		}).
		WithData(t, &pbdemo.Album{
			Name: "foo",
		}).
		Build()

	txn := idx.Txn()
	require.NoError(t, txn.Insert(r1))
	txn.Commit()

	out, err := idx.Txn().Get(r1.Id)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, r1, out)
}

func TestOwnerIndex(t *testing.T) {
	idx := OwnerIndex("test", index.IndexRequired)

	r1 := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		}).
		WithData(t, &pbdemo.Album{
			Name: "foo",
		}).
		WithOwner(&pbresource.ID{
			Type: pbdemo.ArtistType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
			},
		}).
		Build()

	txn := idx.Txn()
	require.NoError(t, txn.Insert(r1))
	txn.Commit()

	out, err := idx.Txn().Get(r1.Owner)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, r1, out)
}

func TestSingleIDOrRefIndex(t *testing.T) {
	getRef := indexersmock.NewGetSingleRefOrID(t)

	idx := SingleIDOrRefIndex("test", getRef.Execute)

	r1 := resourcetest.Resource(pbdemo.AlbumType, "foo").Build()
	ref := &pbresource.Reference{
		Type: pbdemo.ArtistType,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: "foo",
	}

	getRef.EXPECT().Execute(r1).
		Return(ref).
		Once()

	txn := idx.Txn()
	require.NoError(t, txn.Insert(r1))
	txn.Commit()

	out, err := idx.Txn().Get(ref)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, r1, out)
}
