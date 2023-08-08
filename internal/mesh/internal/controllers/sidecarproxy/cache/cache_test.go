package cache

import (
	"testing"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
)

func TestWrite_Create(t *testing.T) {
	cache := New()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination := testDestination(proxyID)
	cache.Write(destination)

	destKey := KeyFromRefAndPort(destination.ServiceRef, destination.Port)
	require.Equal(t, destination, cache.store[destKey])
	actualSourceProxies := cache.sourceProxiesIndex
	expectedSourceProxies := map[string]storeKeys{
		KeyFromID(proxyID): {destKey: struct{}{}},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)

	// Check that we can read back the destination successfully.
	require.Equal(t, destination, cache.ReadDestination(destination.ServiceRef, destination.Port))
}

func TestWrite_Update(t *testing.T) {
	cache := New()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.Write(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.Write(destination2)

	// Check that the source proxies are updated.
	actualSourceProxies := cache.sourceProxiesIndex
	expectedSourceProxies := map[string]storeKeys{
		KeyFromID(proxyID): {
			KeyFromRefAndPort(destination1.ServiceRef, destination1.Port): struct{}{},
			KeyFromRefAndPort(destination2.ServiceRef, destination2.Port): struct{}{},
		},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)

	// Add another destination for a different proxy.
	anotherProxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-def").ID()
	destination3 := testDestination(anotherProxyID)
	destination3.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-3").ReferenceNoSection()
	cache.Write(destination3)

	actualSourceProxies = cache.sourceProxiesIndex
	expectedSourceProxies = map[string]storeKeys{
		KeyFromID(proxyID): {
			KeyFromRefAndPort(destination1.ServiceRef, destination1.Port): struct{}{},
			KeyFromRefAndPort(destination2.ServiceRef, destination2.Port): struct{}{},
		},
		KeyFromID(anotherProxyID): {
			KeyFromRefAndPort(destination3.ServiceRef, destination3.Port): struct{}{},
		},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)
}

func TestWrite_Delete(t *testing.T) {
	cache := New()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.Write(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.Write(destination2)

	cache.Delete(destination1.ServiceRef, destination1.Port)

	require.NotContains(t, cache.store, KeyFromRefAndPort(destination1.ServiceRef, destination1.Port))

	// Check that the source proxies are updated.
	actualSourceProxies := cache.sourceProxiesIndex
	expectedSourceProxies := map[string]storeKeys{
		KeyFromID(proxyID): {
			KeyFromRefAndPort(destination2.ServiceRef, destination2.Port): struct{}{},
		},
	}
	require.Equal(t, expectedSourceProxies, actualSourceProxies)

	// Try to delete non-existing destination and check that nothing has changed..
	cache.Delete(
		resourcetest.Resource(catalog.ServiceType, "does-not-exist").ReferenceNoSection(),
		"doesn't-matter")

	require.Contains(t, cache.store, KeyFromRefAndPort(destination2.ServiceRef, destination2.Port))
	require.Equal(t, expectedSourceProxies, cache.sourceProxiesIndex)
}

func TestDeleteSourceProxy(t *testing.T) {
	cache := New()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.Write(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.Write(destination2)

	cache.DeleteSourceProxy(proxyID)

	// Check that source proxy index is gone.
	proxyKey := KeyFromID(proxyID)
	require.NotContains(t, cache.sourceProxiesIndex, proxyKey)

	// Check that the destinations no longer have this proxy as the source.
	require.NotContains(t, destination1.SourceProxies, proxyKey)
	require.NotContains(t, destination2.SourceProxies, proxyKey)

	// Try to add a non-existent key to source proxy index
	cache.sourceProxiesIndex[proxyKey] = map[string]struct{}{"doesn't-exist": {}}
	cache.DeleteSourceProxy(proxyID)

	// Check that source proxy index is gone.
	require.NotContains(t, cache.sourceProxiesIndex, proxyKey)

	// Check that the destinations no longer have this proxy as the source.
	require.NotContains(t, destination1.SourceProxies, proxyKey)
	require.NotContains(t, destination2.SourceProxies, proxyKey)
}

func TestDestinationsBySourceProxy(t *testing.T) {
	cache := New()

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()
	destination1 := testDestination(proxyID)
	cache.Write(destination1)

	// Add another destination for the same proxy ID.
	destination2 := testDestination(proxyID)
	destination2.ServiceRef = resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection()
	cache.Write(destination2)

	actualDestinations := cache.DestinationsBySourceProxy(proxyID)
	expectedDestinations := []*intermediate.CombinedDestinationRef{destination1, destination2}
	require.ElementsMatch(t, expectedDestinations, actualDestinations)
}

func testDestination(proxyID *pbresource.ID) *intermediate.CombinedDestinationRef {
	return &intermediate.CombinedDestinationRef{
		ServiceRef:             resourcetest.Resource(catalog.ServiceType, "test-service").ReferenceNoSection(),
		Port:                   "tcp",
		ExplicitDestinationsID: resourcetest.Resource(types.UpstreamsType, "test-servicedestinations").ID(),
		SourceProxies: map[string]*pbresource.ID{
			KeyFromID(proxyID): proxyID,
		},
	}
}
