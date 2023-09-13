// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxycache

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
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

	proxyID1 := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	proxyID2 := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-2").
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
	require.Len(t, cache.identityToProxyIDs, 2)
	require.Len(t, cache.proxyIDToIdentity, 2)

	// Untrack proxy 2
	cache.UntrackProxyID(proxyID2)
	require.Equal(t, []*pbresource.ID{proxyID1}, cache.ProxyIDsByWorkloadIdentity(identityID2))
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID1))
	require.Len(t, cache.identityToProxyIDs, 1)
	require.Len(t, cache.proxyIDToIdentity, 1)

	// Untrack proxy 1
	cache.UntrackProxyID(proxyID1)
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID2))
	require.Nil(t, cache.ProxyIDsByWorkloadIdentity(identityID1))
	require.Len(t, cache.identityToProxyIDs, 0)
	require.Len(t, cache.proxyIDToIdentity, 0)
}
