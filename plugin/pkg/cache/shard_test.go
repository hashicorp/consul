package cache

import (
	"sync"
	"testing"
)

func TestShardAddAndGet(t *testing.T) {
	s := newShard(1)
	s.Add(1, 1)

	if _, found := s.Get(1); !found {
		t.Fatal("Failed to find inserted record")
	}

	s.Add(2, 1)
	if _, found := s.Get(1); found {
		t.Fatal("Failed to evict record")
	}
	if _, found := s.Get(2); !found {
		t.Fatal("Failed to find inserted record")
	}
}

func TestAddEvict(t *testing.T) {
	const size = 1024
	s := newShard(size)

	for i := uint64(0); i < size; i++ {
		s.Add(i, 1)
	}
	for i := uint64(0); i < size; i++ {
		s.Add(i, 1)
		if s.Len() != size {
			t.Fatal("A item was unnecessarily evicted from the cache")
		}
	}
}

func TestShardLen(t *testing.T) {
	s := newShard(4)

	s.Add(1, 1)
	if l := s.Len(); l != 1 {
		t.Fatalf("Shard size should %d, got %d", 1, l)
	}

	s.Add(1, 1)
	if l := s.Len(); l != 1 {
		t.Fatalf("Shard size should %d, got %d", 1, l)
	}

	s.Add(2, 2)
	if l := s.Len(); l != 2 {
		t.Fatalf("Shard size should %d, got %d", 2, l)
	}
}

func TestShardEvict(t *testing.T) {
	s := newShard(1)
	s.Add(1, 1)
	s.Add(2, 2)
	// 1 should be gone

	if _, found := s.Get(1); found {
		t.Fatal("Found item that should have been evicted")
	}
}

func TestShardLenEvict(t *testing.T) {
	s := newShard(4)
	s.Add(1, 1)
	s.Add(2, 1)
	s.Add(3, 1)
	s.Add(4, 1)

	if l := s.Len(); l != 4 {
		t.Fatalf("Shard size should %d, got %d", 4, l)
	}

	// This should evict one element
	s.Add(5, 1)
	if l := s.Len(); l != 4 {
		t.Fatalf("Shard size should %d, got %d", 4, l)
	}

	// Make sure we don't accidentally evict an element when
	// we the key is already stored.
	for i := 0; i < 4; i++ {
		s.Add(5, 1)
		if l := s.Len(); l != 4 {
			t.Fatalf("Shard size should %d, got %d", 4, l)
		}
	}
}

func TestShardEvictParallel(t *testing.T) {
	s := newShard(shardSize)
	for i := uint64(0); i < shardSize; i++ {
		s.Add(i, struct{}{})
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < shardSize; i++ {
		wg.Add(1)
		go func() {
			<-start
			s.Evict()
			wg.Done()
		}()
	}
	close(start) // start evicting in parallel
	wg.Wait()
	if s.Len() != 0 {
		t.Fatalf("Failed to evict all keys in parallel: %d", s.Len())
	}
}

func BenchmarkShard(b *testing.B) {
	s := newShard(shardSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := uint64(i) % shardSize * 2
		s.Add(k, 1)
		s.Get(k)
	}
}

func BenchmarkShardParallel(b *testing.B) {
	s := newShard(shardSize)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for i := uint64(0); pb.Next(); i++ {
			k := i % shardSize * 2
			s.Add(k, 1)
			s.Get(k)
		}
	})
}
