// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import "strings"

const defaultModifiedCache = 8192

type Txn[T any] struct {
	tree *RadixTree[T]

	size uint64

	// snap is a snapshot of the node node for use if we have to run the
	// slow notify algorithm.
	snap Node[T]

	// trackChannels is used to hold channels that need to be notified to
	// signal mutation of the tree. This will only hold up to
	// defaultModifiedCache number of entries, after which we will set the
	// trackOverflow flag, which will cause us to use a more expensive
	// algorithm to perform the notifications. Mutation tracking is only
	// performed if trackMutate is true.
	trackChannels map[chan struct{}]struct{}
	trackOverflow bool
	trackMutate   bool
}

// Txn starts a new transaction that can be used to mutate the tree
func (t *RadixTree[T]) Txn() *Txn[T] {
	txn := &Txn[T]{
		size: t.size,
		snap: t.root,
		tree: t,
	}
	return txn
}

// Clone makes an independent copy of the transaction. The new transaction
// does not track any nodes and has TrackMutate turned off. The cloned transaction will contain any uncommitted writes in the original transaction but further mutations to either will be independent and result in different radix trees on Commit. A cloned transaction may be passed to another goroutine and mutated there independently however each transaction may only be mutated in a single thread.
func (t *Txn[T]) Clone() *Txn[T] {
	// reset the writable node cache to avoid leaking future writes into the clone

	txn := &Txn[T]{
		tree: t.tree,
		snap: t.snap,
		size: t.size,
	}
	return txn
}

// TrackMutate can be used to toggle if mutations are tracked. If this is enabled
// then notifications will be issued for affected internal nodes and leaves when
// the transaction is committed.
func (t *Txn[T]) TrackMutate(track bool) {
	t.trackMutate = track
}

// trackChannel safely attempts to track the given mutation channel, setting the
// overflow flag if we can no longer track any more. This limits the amount of
// state that will accumulate during a transaction and we have a slower algorithm
// to switch to if we overflow.
func (t *Txn[T]) trackChannel(ch chan struct{}) {
	// In overflow, make sure we don't store any more objects.
	if t.trackOverflow {
		return
	}

	// If this would overflow the state we reject it and set the flag (since
	// we aren't tracking everything that's required any longer).
	if len(t.trackChannels) >= defaultModifiedCache {
		// Mark that we are in the overflow state
		t.trackOverflow = true

		// Clear the map so that the channels can be garbage collected. It is
		// safe to do this since we have already overflowed and will be using
		// the slow notify algorithm.
		t.trackChannels = nil
		return
	}

	// Create the map on the fly when we need it.
	if t.trackChannels == nil {
		t.trackChannels = make(map[chan struct{}]struct{})
	}

	// Otherwise we are good to track it.
	t.trackChannels[ch] = struct{}{}
}

// Visit all the nodes in the tree under n, and add their mutateChannels to the transaction
// Returns the size of the subtree visited
func (t *Txn[T]) trackChannelsAndCount(n Node[T]) int {
	// Count only leaf nodes
	leaves := 0
	if n.isLeaf() {
		leaves = 1
	}
	// Mark this node as being mutated.
	if t.trackMutate {
		t.trackChannel(n.getMutateCh())
	}

	// Mark its leaf as being mutated, if appropriate.
	if t.trackMutate && n.isLeaf() {
		t.trackChannel(n.getMutateCh())
	}

	// Recurse on the children
	for _, ch := range n.getChildren() {
		leaves += t.trackChannelsAndCount(ch)
	}
	return leaves
}

// Get is used to look up a specific key, returning
// the value and if it was found
func (t *Txn[T]) Get(k []byte) (T, bool) {
	res, found, _ := t.tree.Get(k)
	return res, found
}

func (t *Txn[T]) Insert(key []byte, value T) T {
	oldVal := t.tree.Insert(key, value)
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Delete(key []byte) T {
	oldVal := t.tree.Delete(key)
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Root() Node[T] {
	return t.tree.root
}

// GetWatch is used to lookup a specific key, returning
// the watch channel, value and if it was found
func (t *Txn[T]) GetWatch(k []byte) (<-chan struct{}, T, bool) {
	res, found, watch := t.tree.Get(k)
	return watch, res, found
}

// Notify is used along with TrackMutate to trigger notifications. This must
// only be done once a transaction is committed via CommitOnly, and it is called
// automatically by Commit.
func (t *Txn[T]) Notify() {
	if !t.trackMutate {
		return
	}

	// If we've overflowed the tracking state we can't use it in any way and
	// need to do a full tree compare.
	if t.trackOverflow {
		t.slowNotify()
	} else {
		for ch := range t.trackChannels {
			close(ch)
		}
	}

	// Clean up the tracking state so that a re-notify is safe (will trigger
	// the else clause above which will be a no-op).
	t.trackChannels = nil
	t.trackOverflow = false
}

// Commit is used to finalize the transaction and return a new tree. If mutation
// tracking is turned on then notifications will also be issued.
func (t *Txn[T]) Commit() *RadixTree[T] {
	nt := t.CommitOnly()
	if t.trackMutate {
		t.Notify()
	}
	return nt
}

// CommitOnly is used to finalize the transaction and return a new tree, but
// does not issue any notifications until Notify is called.
func (t *Txn[T]) CommitOnly() *RadixTree[T] {
	nt := &RadixTree[T]{t.tree.root, t.size}
	return nt
}

// slowNotify does a complete comparison of the before and after trees in order
// to trigger notifications. This doesn't require any additional state but it
// is very expensive to compute.
func (t *Txn[T]) slowNotify() {
	snapIter := t.snap.Iterator()
	rootIter := t.Root().Iterator()
	for snapIter.Front() != nil || rootIter.Front() != nil {
		// If we've exhausted the nodes in the old snapshot, we know
		// there's nothing remaining to notify.
		if snapIter.Front() == nil {
			return
		}
		snapElem := snapIter.Front()

		// If we've exhausted the nodes in the new node, we know we need
		// to invalidate everything that remains in the old snapshot. We
		// know from the loop condition there's something in the old
		// snapshot.
		if rootIter.Front() == nil {
			close(snapElem.getMutateCh())
			snapIter.Next()
			continue
		}

		// Do one string compare so we can check the various conditions
		// below without repeating the compare.
		cmp := strings.Compare(snapIter.Path(), rootIter.Path())

		// If the snapshot is behind the node, then we must have deleted
		// this node during the transaction.
		if cmp < 0 {
			close(snapElem.getMutateCh())
			snapIter.Next()
			continue
		}

		// If the snapshot is ahead of the node, then we must have added
		// this node during the transaction.
		if cmp > 0 {
			rootIter.Next()
			continue
		}

		// If we have the same path, then we need to see if we mutated a
		// node and possibly the leaf.
		rootElem := rootIter.Front()
		if snapElem != rootElem {
			close(snapElem.getMutateCh())
		}
		snapIter.Next()
		rootIter.Next()
	}
}
