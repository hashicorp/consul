package cache

import "testing"

func TestCacheAddAndGet(t *testing.T) {
	c := New(4)
	c.Add(1, 1)

	if _, found := c.Get(1); !found {
		t.Fatal("Failed to find inserted record")
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

func BenchmarkCache(b *testing.B) {
	b.ReportAllocs()

	c := New(4)
	for n := 0; n < b.N; n++ {
		c.Add(1, 1)
		c.Get(1)
	}
}
