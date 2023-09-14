// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routestest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

func ReconcileComputedRoutes(
	t *testing.T,
	client pbresource.ResourceServiceClient,
	id *pbresource.ID,
	decResList ...any,
) *types.DecodedComputedRoutes {
	t.Helper()
	if client == nil {
		panic("client is required")
	}
	return makeComputedRoutes(t, client, id, decResList...)
}

func BuildComputedRoutes(
	t *testing.T,
	id *pbresource.ID,
	decResList ...any,
) *types.DecodedComputedRoutes {
	t.Helper()
	return makeComputedRoutes(t, nil, id, decResList...)
}

func MutateTarget(
	t *testing.T,
	portConfig *pbmesh.ComputedPortRoutes,
	ref resource.ReferenceOrID,
	port string,
	mutFn func(details *pbmesh.BackendTargetDetails),
) *pbmesh.ComputedPortRoutes {
	t.Helper()

	backendRef := &pbmesh.BackendReference{
		Ref:        resource.ReferenceFromReferenceOrID(ref),
		Port:       port,
		Datacenter: "",
	}

	key := types.BackendRefToComputedRoutesTarget(backendRef)

	portConfig = proto.Clone(portConfig).(*pbmesh.ComputedPortRoutes)

	details, ok := portConfig.Targets[key]
	require.True(t, ok, "key %q not found in targets map", key)

	mutFn(details)
	return portConfig
}

func makeComputedRoutes(
	t *testing.T,
	client pbresource.ResourceServiceClient,
	id *pbresource.ID,
	decResList ...any,
) *types.DecodedComputedRoutes {
	t.Helper()

	related := loader.NewRelatedResources().
		AddComputedRoutesID(id).
		AddResources(decResList...)

	pending := make(routes.PendingStatuses)
	got := routes.GenerateComputedRoutes(related, pending)
	require.Empty(t, pending, "programmer error if there are warnings")
	require.Len(t, got, 1, "should only compute one output")

	item := got[0]

	if item.Data == nil {
		if client != nil {
			ctx := testutil.TestContext(t)
			_, err := client.Delete(ctx, &pbresource.DeleteRequest{Id: got[0].ID})
			require.NoError(t, err)
		}
		return nil
	}

	b := rtest.ResourceID(id).
		// WithOwner(item.OwnerID).
		WithData(t, item.Data)

	var res *pbresource.Resource
	if client != nil {
		res = b.Write(t, client)
	} else {
		res = b.Build()
	}
	require.NoError(t, types.ValidateComputedRoutes(res))

	return rtest.MustDecode[*pbmesh.ComputedRoutes](t, res)
}
