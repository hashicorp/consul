// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexers

import (
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache/indexers/indexersmock"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestRefOrIDIndex(t *testing.T) {
	ref1 := &pbresource.Reference{
		Type: pbdemo.AlbumType,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: "foo",
	}

	ref2 := &pbresource.Reference{
		Type: pbdemo.AlbumType,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: "bar",
	}

	r1 := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithData(t, &pbdemo.Album{Name: "foo"}).
		Build()

	dec := resourcetest.MustDecode[*pbdemo.Album](t, r1)

	refs := indexersmock.NewRefOrIDFetcher[*pbdemo.Album, *pbresource.Reference](t)

	idx := RefOrIDIndex("test", refs.Execute)

	refs.EXPECT().Execute(dec).
		Return([]*pbresource.Reference{ref1, ref2}).
		Once()

	txn := idx.Txn()
	require.NoError(t, txn.Insert(r1))
	txn.Commit()

	out, err := idx.Txn().Get(ref1)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, r1, out)

	out, err = idx.Txn().Get(ref2)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, r1, out)
}

func TestBoundRefsIndex(t *testing.T) {
	ref1 := &pbresource.Reference{
		Type: pbcatalog.ServiceType,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: "api",
	}

	ref2 := &pbresource.Reference{
		Type: pbcatalog.ServiceType,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: "api-2",
	}

	r1 := resourcetest.Resource(pbmesh.ComputedRoutesType, "api").
		WithData(t, &pbmesh.ComputedRoutes{
			BoundReferences: []*pbresource.Reference{
				ref1,
				ref2,
			},
		}).
		Build()

	idx := BoundRefsIndex[*pbmesh.ComputedRoutes]("test")

	txn := idx.Txn()
	require.NoError(t, txn.Insert(r1))
	txn.Commit()

	out, err := idx.Txn().Get(ref1)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, r1, out)

	out, err = idx.Txn().Get(ref2)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, r1, out)
}
