package cache

import (
	"container/heap"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExpiryHeap_impl(t *testing.T) {
	var _ heap.Interface = new(expiryHeap)
}

func TestExpiryHeap(t *testing.T) {
	require := require.New(t)
	now := time.Now()
	ch := make(chan struct{}, 10) // buffered to prevent blocking in tests
	h := &expiryHeap{NotifyCh: ch}

	// Init, shouldn't trigger anything
	heap.Init(h)
	testNoMessage(t, ch)

	// Push an initial value, expect one message
	entry := &cacheEntryExpiry{Key: "foo", HeapIndex: -1, Expires: now.Add(100)}
	heap.Push(h, entry)
	require.Equal(0, entry.HeapIndex)
	testMessage(t, ch)
	testNoMessage(t, ch) // exactly one asserted above

	// Push another that goes earlier than entry
	entry2 := &cacheEntryExpiry{Key: "bar", HeapIndex: -1, Expires: now.Add(50)}
	heap.Push(h, entry2)
	require.Equal(0, entry2.HeapIndex)
	require.Equal(1, entry.HeapIndex)
	testMessage(t, ch)
	testNoMessage(t, ch) // exactly one asserted above

	// Push another that goes at the end
	entry3 := &cacheEntryExpiry{Key: "bar", HeapIndex: -1, Expires: now.Add(1000)}
	heap.Push(h, entry3)
	require.Equal(2, entry3.HeapIndex)
	testNoMessage(t, ch) // no notify cause index 0 stayed the same

	// Remove the first entry (not Pop, since we don't use Pop, but that works too)
	remove := h.Entries[0]
	heap.Remove(h, remove.HeapIndex)
	require.Equal(0, entry.HeapIndex)
	require.Equal(1, entry3.HeapIndex)
	testMessage(t, ch)
	testMessage(t, ch) // we have two because two swaps happen
	testNoMessage(t, ch)

	// Let's change entry 3 to be early, and fix it
	entry3.Expires = now.Add(10)
	h.Fix(entry3)
	require.Equal(1, entry.HeapIndex)
	require.Equal(0, entry3.HeapIndex)
	testMessage(t, ch)
	testNoMessage(t, ch)

	// Let's change entry 3 again, this is an edge case where if the 0th
	// element changed, we didn't trigger the channel. Our Fix func should.
	entry.Expires = now.Add(20)
	h.Fix(entry3)
	require.Equal(1, entry.HeapIndex) // no move
	require.Equal(0, entry3.HeapIndex)
	testMessage(t, ch)
	testNoMessage(t, ch) // one message
}

func testNoMessage(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal("should not have a message")
	default:
	}
}

func testMessage(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	default:
		t.Fatal("should have a message")
	}
}
