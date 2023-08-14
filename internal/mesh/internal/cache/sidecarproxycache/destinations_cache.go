package sidecarproxycache

import (
	"sync"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// DestinationsCache stores information needed for the sidecar-proxy controller to reconcile efficiently.
// This currently means storing a list of all destinations for easy look up
// as well as indices of source proxies where those destinations are referenced.
//
// It is the responsibility of the controller and its subcomponents (like mapper and data fetcher)
// to keep this cache up-to-date as we're observing new data.
type DestinationsCache struct {
	lock sync.RWMutex

	// store is a map from destination service reference and port as a reference key
	// to the object representing destination reference.
	store map[ReferenceKeyWithPort]intermediate.CombinedDestinationRef

	// sourceProxiesIndex stores a map from a reference key of source proxy IDs
	// to the keys in the store map.
	sourceProxiesIndex map[resource.ReferenceKey]storeKeys
}

type storeKeys map[ReferenceKeyWithPort]struct{}

func NewDestinationsCache() *DestinationsCache {
	return &DestinationsCache{
		store:              make(map[ReferenceKeyWithPort]intermediate.CombinedDestinationRef),
		sourceProxiesIndex: make(map[resource.ReferenceKey]storeKeys),
	}
}

type ReferenceKeyWithPort struct {
	resource.ReferenceKey
	port string
}

func KeyFromRefAndPort(ref *pbresource.Reference, port string) ReferenceKeyWithPort {
	refKey := resource.NewReferenceKey(ref)
	return ReferenceKeyWithPort{refKey, port}
}

// WriteDestination adds destination reference to the cache.
func (c *DestinationsCache) WriteDestination(d intermediate.CombinedDestinationRef) {
	// Check that reference is a catalog.Service type.
	if !resource.EqualType(catalog.ServiceType, d.ServiceRef.Type) {
		panic("ref must of type catalog.Service")
	}

	// Also, check that explicit destination reference is a mesh.Upstreams type.
	if d.ExplicitDestinationsID != nil &&
		!resource.EqualType(types.UpstreamsType, d.ExplicitDestinationsID.Type) {
		panic("ExplicitDestinationsID must be of type mesh.Upstreams")
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.deleteLocked(d.ServiceRef, d.Port)
	c.addLocked(d)
}

// DeleteDestination deletes a given destination reference and port from cache.
func (c *DestinationsCache) DeleteDestination(ref *pbresource.Reference, port string) {
	// Check that reference is a catalog.Service type.
	if !resource.EqualType(catalog.ServiceType, ref.Type) {
		panic("ref must of type catalog.Service")
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.deleteLocked(ref, port)
}

func (c *DestinationsCache) addLocked(d intermediate.CombinedDestinationRef) {
	key := KeyFromRefAndPort(d.ServiceRef, d.Port)

	c.store[key] = d

	// Update source proxies index.
	for proxyRef := range d.SourceProxies {
		_, ok := c.sourceProxiesIndex[proxyRef]
		if !ok {
			c.sourceProxiesIndex[proxyRef] = make(storeKeys)
		}

		c.sourceProxiesIndex[proxyRef][key] = struct{}{}
	}
}

func (c *DestinationsCache) deleteLocked(ref *pbresource.Reference, port string) {
	key := KeyFromRefAndPort(ref, port)

	// First get it from the store.
	dest, ok := c.store[key]
	if !ok {
		// If it's not there, return as there's nothing for us to.
		return
	}

	// Update source proxies indices.
	for proxyRef := range dest.SourceProxies {
		// Delete our destination key from this source proxy.
		delete(c.sourceProxiesIndex[proxyRef], key)
	}

	// Finally, delete this destination from the store.
	delete(c.store, key)
}

// DeleteSourceProxy deletes the source proxy given by id from the cache.
func (c *DestinationsCache) DeleteSourceProxy(id *pbresource.ID) {
	// Check that id is the ProxyStateTemplate type.
	if !resource.EqualType(types.ProxyStateTemplateType, id.Type) {
		panic("id must of type mesh.ProxyStateTemplate")
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	proxyIDKey := resource.NewReferenceKey(id)

	// Get all destination keys.
	destKeys := c.sourceProxiesIndex[proxyIDKey]

	for destKey := range destKeys {
		// Read destination.
		dest, ok := c.store[destKey]
		if !ok {
			// If there's no destination with that key, skip it as there's nothing for us to do.
			continue
		}

		// Delete the source proxy ID.
		delete(dest.SourceProxies, proxyIDKey)
	}

	// Finally, delete the index for this proxy.
	delete(c.sourceProxiesIndex, proxyIDKey)
}

// ReadDestination returns a destination reference for the given service reference and port.
func (c *DestinationsCache) ReadDestination(ref *pbresource.Reference, port string) (intermediate.CombinedDestinationRef, bool) {
	// Check that reference is a catalog.Service type.
	if !resource.EqualType(catalog.ServiceType, ref.Type) {
		panic("ref must of type catalog.Service")
	}

	c.lock.RLock()
	defer c.lock.RUnlock()

	key := KeyFromRefAndPort(ref, port)

	d, found := c.store[key]
	return d, found
}

// DestinationsBySourceProxy returns all destinations that are a referenced by the given source proxy id.
func (c *DestinationsCache) DestinationsBySourceProxy(id *pbresource.ID) []intermediate.CombinedDestinationRef {
	// Check that id is the ProxyStateTemplate type.
	if !resource.EqualType(types.ProxyStateTemplateType, id.Type) {
		panic("id must of type mesh.ProxyStateTemplate")
	}

	c.lock.RLock()
	defer c.lock.RUnlock()

	var destinations []intermediate.CombinedDestinationRef

	proxyIDKey := resource.NewReferenceKey(id)

	for destKey := range c.sourceProxiesIndex[proxyIDKey] {
		destinations = append(destinations, c.store[destKey])
	}

	return destinations
}
