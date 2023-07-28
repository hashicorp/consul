package cache

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Cache stores information needed for the mesh controller to reconcile efficiently.
// This currently means storing a list of all destinations for easy look up
// as well as indices of source proxies where those destinations are referenced.
//
// It is the responsibility of controller and its subcomponents (like mapper and data fetcher)
// to keep this cache up-to-date as we're observing new data.
type Cache struct {
	lock sync.RWMutex

	// store is a map from destination service reference and port as a string ID
	// to the object representing destination reference.
	store map[string]*intermediate.CombinedDestinationRef

	// sourceProxiesIndex stores a map from a string representation of source proxy ID
	// to the keys in the store map.
	sourceProxiesIndex map[string]storeKeys
}

type storeKeys map[string]struct{}

func New() *Cache {
	return &Cache{
		store:              make(map[string]*intermediate.CombinedDestinationRef),
		sourceProxiesIndex: make(map[string]storeKeys),
	}
}

func KeyFromID(id *pbresource.ID) string {
	return fmt.Sprintf("%s/%s/%s",
		resource.ToGVK(id.Type),
		resource.TenancyToString(id.Tenancy),
		id.Name)
}

func KeyFromRefAndPort(ref *pbresource.Reference, port string) string {
	return fmt.Sprintf("%s:%s",
		resource.ReferenceToString(ref),
		port)
}

func (c *Cache) Write(d *intermediate.CombinedDestinationRef) {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := KeyFromRefAndPort(d.ServiceRef, d.Port)

	c.store[key] = d

	// Update source proxies index.
	for _, proxyID := range d.SourceProxies {
		proxyIDKey := KeyFromID(proxyID)

		_, ok := c.sourceProxiesIndex[proxyIDKey]
		if !ok {
			c.sourceProxiesIndex[proxyIDKey] = make(storeKeys)
		}

		c.sourceProxiesIndex[proxyIDKey][key] = struct{}{}
	}
}

func (c *Cache) Delete(ref *pbresource.Reference, port string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := KeyFromRefAndPort(ref, port)

	// First get it from the store.
	dest, ok := c.store[key]
	if !ok {
		// If it's not there, return as there's nothing for us to.
		return
	}

	// Update source proxies indices.
	for _, proxyID := range dest.SourceProxies {
		proxyIDKey := KeyFromID(proxyID)

		// Delete our destination key from this source proxy.
		delete(c.sourceProxiesIndex[proxyIDKey], key)
	}

	// Finally, delete this destination from the store.
	delete(c.store, key)
}

func (c *Cache) DeleteSourceProxy(id *pbresource.ID) {
	c.lock.Lock()
	defer c.lock.Unlock()

	proxyIDKey := KeyFromID(id)

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

func (c *Cache) ReadDestination(ref *pbresource.Reference, port string) *intermediate.CombinedDestinationRef {
	c.lock.RLock()
	defer c.lock.RUnlock()

	key := KeyFromRefAndPort(ref, port)
	return c.store[key]
}

func (c *Cache) DestinationsBySourceProxy(id *pbresource.ID) []*intermediate.CombinedDestinationRef {
	c.lock.RLock()
	defer c.lock.RUnlock()

	var destinations []*intermediate.CombinedDestinationRef

	proxyIDKey := KeyFromID(id)

	for destKey := range c.sourceProxiesIndex[proxyIDKey] {
		destinations = append(destinations, c.store[destKey])
	}

	return destinations
}
