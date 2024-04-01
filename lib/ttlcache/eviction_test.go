// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ttlcache

import (
	"container/heap"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/sdk/testutil"
)

var _ heap.Interface = (*entryHeap)(nil)

func TestExpiryHeap(t *testing.T) {
	h := NewExpiryHeap()
	ch := h.NotifyCh
	var entry, entry2, entry3 *Entry

	// Init, shouldn't trigger anything
	testNoMessage(t, ch)

	testutil.RunStep(t, "add an entry", func(t *testing.T) {
		entry = h.Add("foo", 100*time.Millisecond)
		assert.Equal(t, 0, entry.heapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch) // exactly one asserted above
	})

	testutil.RunStep(t, "add a second entry in front", func(t *testing.T) {
		entry2 = h.Add("bar", 50*time.Millisecond)
		assert.Equal(t, 0, entry2.heapIndex)
		assert.Equal(t, 1, entry.heapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch) // exactly one asserted above
	})

	testutil.RunStep(t, "add a third entry at the end", func(t *testing.T) {
		entry3 = h.Add("baz", 1000*time.Millisecond)
		assert.Equal(t, 2, entry3.heapIndex)
		testNoMessage(t, ch) // no notify cause index 0 stayed the same
	})

	testutil.RunStep(t, "remove the first entry", func(t *testing.T) {
		h.Remove(0)
		assert.Equal(t, 0, entry.heapIndex)
		assert.Equal(t, 1, entry3.heapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch)
	})

	testutil.RunStep(t, "update so that entry3 expires first", func(t *testing.T) {
		h.Update(entry.heapIndex, 2000*time.Millisecond)
		assert.Equal(t, 1, entry.heapIndex)
		assert.Equal(t, 0, entry3.heapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch)
	})

	testutil.RunStep(t, "0th element change triggers a notify", func(t *testing.T) {
		h.Update(entry3.heapIndex, 1500*time.Millisecond)
		assert.Equal(t, 1, entry.heapIndex) // no move
		assert.Equal(t, 0, entry3.heapIndex)
		testMessage(t, ch)
		testNoMessage(t, ch) // one message
	})

	testutil.RunStep(t, "update can not decrease expiry time", func(t *testing.T) {
		h.Update(entry.heapIndex, 100*time.Millisecond)
		assert.Equal(t, 1, entry.heapIndex) // no move
		assert.Equal(t, 0, entry3.heapIndex)
		testNoMessage(t, ch) // no notify, because no change in the heap
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
