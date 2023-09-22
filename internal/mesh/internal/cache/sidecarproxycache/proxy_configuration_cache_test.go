// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxycache

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestProxyConfigurationCache(t *testing.T) {
	cache := NewProxyConfigurationCache()

	// Create some proxy configurations.
	proxyCfg1 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "test-cfg-1").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	proxyCfg2 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "test-cfg-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	proxyCfg3 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "test-cfg-3").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	// Create some proxy state templates.
	p1 := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "w-111").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	p2 := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "w-222").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	p3 := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "w-333").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	p4 := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "w-444").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	p5 := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "w-555").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()

	// Track these and make sure there's some overlap.
	cache.TrackProxyConfiguration(proxyCfg1, []resource.ReferenceOrID{p1, p2, p4})
	cache.TrackProxyConfiguration(proxyCfg2, []resource.ReferenceOrID{p3, p4, p5})
	cache.TrackProxyConfiguration(proxyCfg3, []resource.ReferenceOrID{p1, p3})

	// Read proxy configurations by proxy.
	requireProxyConfigurations(t, cache, p1, proxyCfg1, proxyCfg3)
	requireProxyConfigurations(t, cache, p2, proxyCfg1)
	requireProxyConfigurations(t, cache, p3, proxyCfg2, proxyCfg3)
	requireProxyConfigurations(t, cache, p4, proxyCfg1, proxyCfg2)
	requireProxyConfigurations(t, cache, p5, proxyCfg2)

	// Untrack some proxy IDs.
	cache.UntrackProxyID(p1)

	requireProxyConfigurations(t, cache, p1)

	// Untrack some proxy IDs.
	cache.UntrackProxyID(p3)

	requireProxyConfigurations(t, cache, p3)

	// Untrack proxy cfg.
	cache.UntrackProxyConfiguration(proxyCfg1)

	requireProxyConfigurations(t, cache, p1) // no-op because we untracked it earlier
	requireProxyConfigurations(t, cache, p2)
	requireProxyConfigurations(t, cache, p3) // no-op because we untracked it earlier
	requireProxyConfigurations(t, cache, p4, proxyCfg2)
	requireProxyConfigurations(t, cache, p5, proxyCfg2)
}

func requireProxyConfigurations(t *testing.T, cache *ProxyConfigurationCache, proxyID *pbresource.ID, proxyCfgs ...*pbresource.ID) {
	t.Helper()

	actualProxyCfgs := cache.ProxyConfigurationsByProxyID(proxyID)

	require.Len(t, actualProxyCfgs, len(proxyCfgs))
	prototest.AssertElementsMatch(t, proxyCfgs, actualProxyCfgs)
}
