// Package cache implements a cache. The cache hold 256 shards, each shard
// holds a cache: a map with a mutex. There is no fancy expunge algorithm, it
// just randomly evicts elements when it gets full.
package cache

import (
	"hash/fnv"
	"sync"
)

// Hash returns the FNV hash of what.
func Hash(what []byte) uint32 {
	h := fnv.New32()
	h.Write(what)
	return h.Sum32()
}

// Cache is cache.
type Cache struct {
	shards [shardSize]*shard
}

// shard is a cache with random eviction.
type shard struct {
	items map[uint32]interface{}
	size  int

	sync.RWMutex
}

// New returns a new cache.
func New(size int) *Cache {
	ssize := size / shardSize
	if ssize < 512 {
		ssize = 512
	}

	c := &Cache{}

	// Initialize all the shards
	for i := 0; i < shardSize; i++ {
		c.shards[i] = newShard(ssize)
	}
	return c
}

// Add adds a new element to the cache. If the element already exists it is overwritten.
func (c *Cache) Add(key uint32, el interface{}) {
	shard := key & (shardSize - 1)
	c.shards[shard].Add(key, el)
}

// Get looks up element index under key.
func (c *Cache) Get(key uint32) (interface{}, bool) {
	shard := key & (shardSize - 1)
	return c.shards[shard].Get(key)
}

// Remove removes the element indexed with key.
func (c *Cache) Remove(key uint32) {
	shard := key & (shardSize - 1)
	c.shards[shard].Remove(key)
}

// Len returns the number of elements in the cache.
func (c *Cache) Len() int {
	l := 0
	for _, s := range c.shards {
		l += s.Len()
	}
	return l
}

// newShard returns a new shard with size.
func newShard(size int) *shard { return &shard{items: make(map[uint32]interface{}), size: size} }

// Add adds element indexed by key into the cache. Any existing element is overwritten
func (s *shard) Add(key uint32, el interface{}) {
	l := s.Len()
	if l+1 > s.size {
		s.Evict()
	}

	s.Lock()
	s.items[key] = el
	s.Unlock()
}

// Remove removes the element indexed by key from the cache.
func (s *shard) Remove(key uint32) {
	s.Lock()
	delete(s.items, key)
	s.Unlock()
}

// Evict removes a random element from the cache.
func (s *shard) Evict() {
	key := -1

	s.RLock()
	for k := range s.items {
		key = int(k)
		break
	}
	s.RUnlock()

	if key == -1 {
		// empty cache
		return
	}

	// If this item is gone between the RUnlock and Lock race we don't care.
	s.Remove(uint32(key))
}

// Get looks up the element indexed under key.
func (s *shard) Get(key uint32) (interface{}, bool) {
	s.RLock()
	el, found := s.items[key]
	s.RUnlock()
	return el, found
}

// Len returns the current length of the cache.
func (s *shard) Len() int {
	s.RLock()
	l := len(s.items)
	s.RUnlock()
	return l
}

const shardSize = 256
