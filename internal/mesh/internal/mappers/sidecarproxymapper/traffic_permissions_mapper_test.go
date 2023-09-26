// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMapComputedTrafficPermissionsToProxyStateTemplate(t *testing.T) {
	client := svctest.RunResourceService(t, types.Register, catalog.RegisterTypes)
	ctp := resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, "workload-identity-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbauth.ComputedTrafficPermissions{}).
		Build()

	i := sidecarproxycache.NewIdentitiesCache()
	mapper := &Mapper{identitiesCache: i}

	// Empty results when the cache isn't populated.
	requests, err := mapper.MapComputedTrafficPermissionsToProxyStateTemplate(context.Background(), controller.Runtime{Client: client}, ctp)
	require.NoError(t, err)
	require.Len(t, requests, 0)

	identityID1 := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	proxyID1 := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "service-workload-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	proxyID2 := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "service-workload-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	i.TrackPair(identityID1, proxyID1)

	// Empty results when the cache isn't populated.
	requests, err = mapper.MapComputedTrafficPermissionsToProxyStateTemplate(context.Background(), controller.Runtime{Client: client}, ctp)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, []controller.Request{{ID: proxyID1}}, requests)

	i.TrackPair(identityID1, proxyID2)

	// Empty results when the cache isn't populated.
	requests, err = mapper.MapComputedTrafficPermissionsToProxyStateTemplate(context.Background(), controller.Runtime{Client: client}, ctp)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t, []controller.Request{
		{ID: proxyID1},
		{ID: proxyID2},
	}, requests)
}
