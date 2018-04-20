package cache

import (
	"container/heap"
	"testing"
)

func TestExpiryHeap_impl(t *testing.T) {
	var _ heap.Interface = new(expiryHeap)
}
