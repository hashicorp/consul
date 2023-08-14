package sidecarproxycache

import (
	"testing"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
)

func TestWrite_Create(t *testing.T) {
	cache := NewDestinationsCache()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination := testDestination(proxyID)
	cache.WriteDestination(destination)

	destKey := KeyFromRefAndPort(destination.ServiceRef, destination.Port)
	require.Equal(t, destination, cache.store[destKey])
	actualSourceProxies := cache.sourceProxiesIndex
	expectedSourceProxies := map[resource.ReferenceKey]storeKeys{
		resource.NewReferenceKey(proxyID): {destKey: struct{}{}},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)

	// Check that we can read back the destination successfully.
	actualDestination, found := cache.ReadDestination(destination.ServiceRef, destination.Port)
	require.True(t, found)
	require.Equal(t, destination, actualDestination)
}

func TestWrite_Update(t *testing.T) {
	cache := NewDestinationsCache()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.WriteDestination(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.WriteDestination(destination2)

	// Check that the source proxies are updated.
	actualSourceProxies := cache.sourceProxiesIndex
	expectedSourceProxies := map[resource.ReferenceKey]storeKeys{
		resource.NewReferenceKey(proxyID): {
			KeyFromRefAndPort(destination1.ServiceRef, destination1.Port): struct{}{},
			KeyFromRefAndPort(destination2.ServiceRef, destination2.Port): struct{}{},
		},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)

	// Add another destination for a different proxy.
	anotherProxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-def").ID()
	destination3 := testDestination(anotherProxyID)
	destination3.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-3").ReferenceNoSection()
	cache.WriteDestination(destination3)

	actualSourceProxies = cache.sourceProxiesIndex
	expectedSourceProxies = map[resource.ReferenceKey]storeKeys{
		resource.NewReferenceKey(proxyID): {
			KeyFromRefAndPort(destination1.ServiceRef, destination1.Port): struct{}{},
			KeyFromRefAndPort(destination2.ServiceRef, destination2.Port): struct{}{},
		},
		resource.NewReferenceKey(anotherProxyID): {
			KeyFromRefAndPort(destination3.ServiceRef, destination3.Port): struct{}{},
		},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)

	// Overwrite the proxy id completely.
	destination1.SourceProxies = map[resource.ReferenceKey]struct{}{resource.NewReferenceKey(anotherProxyID): {}}
	cache.WriteDestination(destination1)

	actualSourceProxies = cache.sourceProxiesIndex
	expectedSourceProxies = map[resource.ReferenceKey]storeKeys{
		resource.NewReferenceKey(proxyID): {
			KeyFromRefAndPort(destination2.ServiceRef, destination2.Port): struct{}{},
		},
		resource.NewReferenceKey(anotherProxyID): {
			KeyFromRefAndPort(destination1.ServiceRef, destination1.Port): struct{}{},
			KeyFromRefAndPort(destination3.ServiceRef, destination3.Port): struct{}{},
		},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)
}

func TestWrite_Delete(t *testing.T) {
	cache := NewDestinationsCache()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.WriteDestination(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.WriteDestination(destination2)

	cache.DeleteDestination(destination1.ServiceRef, destination1.Port)

	require.NotContains(t, cache.store, KeyFromRefAndPort(destination1.ServiceRef, destination1.Port))

	// Check that the source proxies are updated.
	actualSourceProxies := cache.sourceProxiesIndex
	expectedSourceProxies := map[resource.ReferenceKey]storeKeys{
		resource.NewReferenceKey(proxyID): {
			KeyFromRefAndPort(destination2.ServiceRef, destination2.Port): struct{}{},
		},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)

	// Try to delete non-existing destination and check that nothing has changed..
	cache.DeleteDestination(
		resourcetest.Resource(catalog.ServiceType, "does-not-exist").ReferenceNoSection(),
		"doesn't-matter")

	require.Contains(t, cache.store, KeyFromRefAndPort(destination2.ServiceRef, destination2.Port))
	require.Equal(t, expectedSourceProxies, cache.sourceProxiesIndex)
}

func TestDeleteSourceProxy(t *testing.T) {
	cache := NewDestinationsCache()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.WriteDestination(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.WriteDestination(destination2)

	cache.DeleteSourceProxy(proxyID)

	// Check that source proxy index is gone.
	proxyKey := resource.NewReferenceKey(proxyID)
	require.NotContains(t, cache.sourceProxiesIndex, proxyKey)

	// Check that the destinations no longer have this proxy as the source.
	require.NotContains(t, destination1.SourceProxies, proxyKey)
	require.NotContains(t, destination2.SourceProxies, proxyKey)

	// Try to add a non-existent key to source proxy index
	cache.sourceProxiesIndex[proxyKey] = map[ReferenceKeyWithPort]struct{}{
		{port: "doesn't-matter"}: {}}
	cache.DeleteSourceProxy(proxyID)

	// Check that source proxy index is gone.
	require.NotContains(t, cache.sourceProxiesIndex, proxyKey)

	// Check that the destinations no longer have this proxy as the source.
	require.NotContains(t, destination1.SourceProxies, proxyKey)
	require.NotContains(t, destination2.SourceProxies, proxyKey)
}

func TestDestinationsBySourceProxy(t *testing.T) {
	cache := NewDestinationsCache()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.WriteDestination(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.WriteDestination(destination2)

	actualDestinations := cache.DestinationsBySourceProxy(proxyID)
	expectedDestinations := []intermediate.CombinedDestinationRef{destination1, destination2}
	require.ElementsMatch(t, expectedDestinations, actualDestinations)
}

func testDestination(proxyID *pbresource.ID) intermediate.CombinedDestinationRef {
	return intermediate.CombinedDestinationRef{
		ServiceRef:             resourcetest.Resource(catalog.ServiceType, "test-service").ReferenceNoSection(),
		Port:                   "tcp",
		ExplicitDestinationsID: resourcetest.Resource(types.UpstreamsType, "test-servicedestinations").ID(),
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(proxyID): {},
		},
	}
}
