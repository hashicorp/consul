package cache

import "testing"

func TestCacheAddAndGet(t *testing.T) {
	const N = shardSize * 4
	c := New(N)
	c.Add(1, 1)

	if _, found := c.Get(1); !found {
		t.Fatal("Failed to find inserted record")
	}

	for i := 0; i < N; i++ {
		c.Add(uint64(i), 1)
	}
	for i := 0; i < N; i++ {
		c.Add(uint64(i), 1)
		if c.Len() != N {
			t.Fatal("A item was unnecessarily evicted from the cache")
		}
	}
}

func TestCacheLen(t *testing.T) {
	c := New(4)

	c.Add(1, 1)
	if l := c.Len(); l != 1 {
		t.Fatalf("Cache size should %d, got %d", 1, l)
	}

	c.Add(1, 1)
	if l := c.Len(); l != 1 {
		t.Fatalf("Cache size should %d, got %d", 1, l)
	}

	c.Add(2, 2)
	if l := c.Len(); l != 2 {
		t.Fatalf("Cache size should %d, got %d", 2, l)
	}
}

func TestCacheSharding(t *testing.T) {
	c := New(shardSize)
	for i := 0; i < shardSize*2; i++ {
		c.Add(uint64(i), 1)
	}
	for i, s := range c.shards {
		if s.Len() == 0 {
			t.Errorf("Failed to populate shard: %d", i)
		}
	}
}

func BenchmarkCache(b *testing.B) {
	b.ReportAllocs()

	c := New(4)
	for n := 0; n < b.N; n++ {
		c.Add(1, 1)
		c.Get(1)
	}
}
