package cache

import "testing"

func TestShardAddAndGet(t *testing.T) {
	s := newShard(4)
	s.Add(1, 1)

	if _, found := s.Get(1); !found {
		t.Fatal("Failed to find inserted record")
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
}
