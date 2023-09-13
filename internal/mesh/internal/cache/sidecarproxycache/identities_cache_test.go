// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxycache

import (
	"testing"

	"github.com/stretchr/testify/require"

	meshapi "github.com/hashicorp/consul/api/mesh/v2beta1"
	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestIdentitiesCache(t *testing.T) {
	cache := NewIdentitiesCache()

	identityID1 := resourcetest.Resource(auth.WorkloadIdentityType, "workload-identity-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	identityID2 := resourcetest.Resource(auth.WorkloadIdentityType, "workload-identity-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	proxyID1 := resourcetest.Resource(meshapi.ProxyStateTemplateType, "service-workload-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	proxyID2 := resourcetest.Resource(meshapi.ProxyStateTemplateType, "service-workload-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	// Empty cache
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID1))
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID2))

	// Insert value and fetch it.
	cache.TrackPair(identityID1, proxyID1)
	require.Equal(t, []*pbresource.ID{proxyID1}, cache.ProxyIDsByWorkloadIdentity(identityID1))
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID2))

	// Insert another value referencing the same identity.
	cache.TrackPair(identityID1, proxyID2)
	require.ElementsMatch(t, []*pbresource.ID{proxyID1, proxyID2}, cache.ProxyIDsByWorkloadIdentity(identityID1))
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID2))

	// Now proxy 1 uses identity 2
	cache.TrackPair(identityID2, proxyID1)
	require.Equal(t, []*pbresource.ID{proxyID1}, cache.ProxyIDsByWorkloadIdentity(identityID2))
	require.Equal(t, []*pbresource.ID{proxyID2}, cache.ProxyIDsByWorkloadIdentity(identityID1))

	// Untrack proxy 2
	cache.UntrackProxyID(proxyID2)
	require.Equal(t, []*pbresource.ID{proxyID1}, cache.ProxyIDsByWorkloadIdentity(identityID2))
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID1))

	// Untrack proxy 1
	cache.UntrackProxyID(proxyID1)
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID2))
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID1))
}
