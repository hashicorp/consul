package cache

import (
	"container/heap"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ heap.Interface = new(expiryHeap)

func TestExpiryHeap(t *testing.T) {
	require := require.New(t)
	ch := make(chan struct{}, 10) // buffered to prevent blocking in tests
	h := &expiryHeap{NotifyCh: ch}
	var entry, entry2, entry3 *cacheEntryExpiry

	// Init, shouldn't trigger anything
	heap.Init(h)
	testNoMessage(t, ch)

	runStep(t, "add an entry", func(t *testing.T) {
		entry = h.Add("foo", 100*time.Millisecond)
		require.Equal(0, entry.HeapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch) // exactly one asserted above
	})

	runStep(t, "add a second entry in front", func(t *testing.T) {
		entry2 = h.Add("bar", 50*time.Millisecond)
		require.Equal(0, entry2.HeapIndex)
		require.Equal(1, entry.HeapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch) // exactly one asserted above
	})

	runStep(t, "add a third entry at the end", func(t *testing.T) {
		entry3 = h.Add("baz", 1000*time.Millisecond)
		require.Equal(2, entry3.HeapIndex)
		testNoMessage(t, ch) // no notify cause index 0 stayed the same
	})

	runStep(t, "remove the first entry", func(t *testing.T) {
		h.Remove(0)
		require.Equal(0, entry.HeapIndex)
		require.Equal(1, entry3.HeapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch)
	})

	runStep(t, "update entry3 to expire first", func(t *testing.T) {
		h.Update(entry3.HeapIndex, 10*time.Millisecond)
		assert.Equal(t, 1, entry.HeapIndex)
		assert.Equal(t, 0, entry3.HeapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch)
	})

	runStep(t, "0th element change triggers a notify", func(t *testing.T) {
		h.Update(entry3.HeapIndex, 20)
		require.Equal(1, entry.HeapIndex) // no move
		require.Equal(0, entry3.HeapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch) // one message
	})
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

func runStep(t *testing.T, name string, fn func(t *testing.T)) {
	if !t.Run(name, fn) {
		t.FailNow()
	}
}

func TestExpiryLoop_ExitsWhenStopped(t *testing.T) {
	c := &Cache{
		stopCh:            make(chan struct{}),
		entries:           make(map[string]cacheEntry),
		entriesExpiryHeap: newExpiryHeap(),
	}
	chStart := make(chan struct{})
	chDone := make(chan struct{})
	go func() {
		close(chStart)
		c.runExpiryLoop()
		close(chDone)
	}()

	<-chStart
	close(c.stopCh)

	select {
	case <-chDone:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected loop to exit when stopped")
	}
}
