package iradix

import (
	"bytes"
	"strings"

	"github.com/hashicorp/golang-lru/simplelru"
)

const (
	// defaultModifiedCache is the default size of the modified node
	// cache used per transaction. This is used to cache the updates
	// to the nodes near the root, while the leaves do not need to be
	// cached. This is important for very large transactions to prevent
	// the modified cache from growing to be enormous. This is also used
	// to set the max size of the mutation notify maps since those should
	// also be bounded in a similar way.
	defaultModifiedCache = 8192
)

// Tree implements an immutable radix tree. This can be treated as a
// Dictionary abstract data type. The main advantage over a standard
// hash map is prefix-based lookups and ordered iteration. The immutability
// means that it is safe to concurrently read from a Tree without any
// coordination.
type Tree struct {
	root *Node
	size int
}

// New returns an empty Tree
func New() *Tree {
	t := &Tree{
		root: &Node{
			mutateCh: make(chan struct{}),
		},
	}
	return t
}

// Len is used to return the number of elements in the tree
func (t *Tree) Len() int {
	return t.size
}

// Txn is a transaction on the tree. This transaction is applied
// atomically and returns a new tree when committed. A transaction
// is not thread safe, and should only be used by a single goroutine.
type Txn struct {
	// root is the modified root for the transaction.
	root *Node

	// snap is a snapshot of the root node for use if we have to run the
	// slow notify algorithm.
	snap *Node

	// size tracks the size of the tree as it is modified during the
	// transaction.
	size int

	// writable is a cache of writable nodes that have been created during
	// the course of the transaction. This allows us to re-use the same
	// nodes for further writes and avoid unnecessary copies of nodes that
	// have never been exposed outside the transaction. This will only hold
	// up to defaultModifiedCache number of entries.
	writable *simplelru.LRU

	// trackChannels is used to hold channels that need to be notified to
	// signal mutation of the tree. This will only hold up to
	// defaultModifiedCache number of entries, after which we will set the
	// trackOverflow flag, which will cause us to use a more expensive
	// algorithm to perform the notifications. Mutation tracking is only
	// performed if trackMutate is true.
	trackChannels map[*chan struct{}]struct{}
	trackOverflow bool
	trackMutate   bool
}

// Txn starts a new transaction that can be used to mutate the tree
func (t *Tree) Txn() *Txn {
	txn := &Txn{
		root: t.root,
		snap: t.root,
		size: t.size,
	}
	return txn
}

// TrackMutate can be used to toggle if mutations are tracked. If this is enabled
// then notifications will be issued for affected internal nodes and leaves when
// the transaction is committed.
func (t *Txn) TrackMutate(track bool) {
	t.trackMutate = track
}

// trackChannel safely attempts to track the given mutation channel, setting the
// overflow flag if we can no longer track any more. This limits the amount of
// state that will accumulate during a transaction and we have a slower algorithm
// to switch to if we overflow.
func (t *Txn) trackChannel(ch *chan struct{}) {
	// In overflow, make sure we don't store any more objects.
	if t.trackOverflow {
		return
	}

	// Create the map on the fly when we need it.
	if t.trackChannels == nil {
		t.trackChannels = make(map[*chan struct{}]struct{})
	}

	// If this would overflow the state we reject it and set the flag (since
	// we aren't tracking everything that's required any longer).
	if len(t.trackChannels) >= defaultModifiedCache {
		t.trackOverflow = true
		return
	}

	// Otherwise we are good to track it.
	t.trackChannels[ch] = struct{}{}
}

// writeNode returns a node to be modified, if the current node has already been
// modified during the course of the transaction, it is used in-place. Set
// forLeafUpdate to true if you are getting a write node to update the leaf,
// which will set leaf mutation tracking appropriately as well.
func (t *Txn) writeNode(n *Node, forLeafUpdate bool) *Node {
	// Ensure the writable set exists.
	if t.writable == nil {
		lru, err := simplelru.NewLRU(defaultModifiedCache, nil)
		if err != nil {
			panic(err)
		}
		t.writable = lru
	}

	// If this node has already been modified, we can continue to use it
	// during this transaction. If a node gets kicked out of cache then we
	// *may* notify for its mutation if we end up copying the node again,
	// but we don't make any guarantees about notifying for intermediate
	// mutations that were never exposed outside of a transaction.
	if _, ok := t.writable.Get(n); ok {
		return n
	}

	// Mark this node as being mutated.
	if t.trackMutate {
		t.trackChannel(&(n.mutateCh))
	}

	// Mark its leaf as being mutated, if appropriate.
	if t.trackMutate && forLeafUpdate && n.leaf != nil {
		t.trackChannel(&(n.leaf.mutateCh))
	}

	// Copy the existing node.
	nc := &Node{
		mutateCh: make(chan struct{}),
		leaf:     n.leaf,
	}
	if n.prefix != nil {
		nc.prefix = make([]byte, len(n.prefix))
		copy(nc.prefix, n.prefix)
	}
	if len(n.edges) != 0 {
		nc.edges = make([]edge, len(n.edges))
		copy(nc.edges, n.edges)
	}

	// Mark this node as writable.
	t.writable.Add(nc, nil)
	return nc
}

// insert does a recursive insertion
func (t *Txn) insert(n *Node, k, search []byte, v interface{}) (*Node, interface{}, bool) {
	// Handle key exhaustion
	if len(search) == 0 {
		var oldVal interface{}
		didUpdate := false
		if n.isLeaf() {
			oldVal = n.leaf.val
			didUpdate = true
		}

		nc := t.writeNode(n, true)
		nc.leaf = &leafNode{
			mutateCh: make(chan struct{}),
			key:      k,
			val:      v,
		}
		return nc, oldVal, didUpdate
	}

	// Look for the edge
	idx, child := n.getEdge(search[0])

	// No edge, create one
	if child == nil {
		e := edge{
			label: search[0],
			node: &Node{
				mutateCh: make(chan struct{}),
				leaf: &leafNode{
					mutateCh: make(chan struct{}),
					key:      k,
					val:      v,
				},
				prefix: search,
			},
		}
		nc := t.writeNode(n, false)
		nc.addEdge(e)
		return nc, nil, false
	}

	// Determine longest prefix of the search key on match
	commonPrefix := longestPrefix(search, child.prefix)
	if commonPrefix == len(child.prefix) {
		search = search[commonPrefix:]
		newChild, oldVal, didUpdate := t.insert(child, k, search, v)
		if newChild != nil {
			nc := t.writeNode(n, false)
			nc.edges[idx].node = newChild
			return nc, oldVal, didUpdate
		}
		return nil, oldVal, didUpdate
	}

	// Split the node
	nc := t.writeNode(n, false)
	splitNode := &Node{
		mutateCh: make(chan struct{}),
		prefix:   search[:commonPrefix],
	}
	nc.replaceEdge(edge{
		label: search[0],
		node:  splitNode,
	})

	// Restore the existing child node
	modChild := t.writeNode(child, false)
	splitNode.addEdge(edge{
		label: modChild.prefix[commonPrefix],
		node:  modChild,
	})
	modChild.prefix = modChild.prefix[commonPrefix:]

	// Create a new leaf node
	leaf := &leafNode{
		mutateCh: make(chan struct{}),
		key:      k,
		val:      v,
	}

	// If the new key is a subset, add to to this node
	search = search[commonPrefix:]
	if len(search) == 0 {
		splitNode.leaf = leaf
		return nc, nil, false
	}

	// Create a new edge for the node
	splitNode.addEdge(edge{
		label: search[0],
		node: &Node{
			mutateCh: make(chan struct{}),
			leaf:     leaf,
			prefix:   search,
		},
	})
	return nc, nil, false
}

// delete does a recursive deletion
func (t *Txn) delete(parent, n *Node, search []byte) (*Node, *leafNode) {
	// Check for key exhaustion
	if len(search) == 0 {
		if !n.isLeaf() {
			return nil, nil
		}

		// Remove the leaf node
		nc := t.writeNode(n, true)
		nc.leaf = nil

		// Check if this node should be merged
		if n != t.root && len(nc.edges) == 1 {
			nc.mergeChild()
		}
		return nc, n.leaf
	}

	// Look for an edge
	label := search[0]
	idx, child := n.getEdge(label)
	if child == nil || !bytes.HasPrefix(search, child.prefix) {
		return nil, nil
	}

	// Consume the search prefix
	search = search[len(child.prefix):]
	newChild, leaf := t.delete(n, child, search)
	if newChild == nil {
		return nil, nil
	}

	// Copy this node. WATCH OUT - it's safe to pass "false" here because we
	// will only ADD a leaf via nc.mergeChilde() if there isn't one due to
	// the !nc.isLeaf() check in the logic just below. This is pretty subtle,
	// so be careful if you change any of the logic here.
	nc := t.writeNode(n, false)

	// Delete the edge if the node has no edges
	if newChild.leaf == nil && len(newChild.edges) == 0 {
		nc.delEdge(label)
		if n != t.root && len(nc.edges) == 1 && !nc.isLeaf() {
			nc.mergeChild()
		}
	} else {
		nc.edges[idx].node = newChild
	}
	return nc, leaf
}

// Insert is used to add or update a given key. The return provides
// the previous value and a bool indicating if any was set.
func (t *Txn) Insert(k []byte, v interface{}) (interface{}, bool) {
	newRoot, oldVal, didUpdate := t.insert(t.root, k, k, v)
	if newRoot != nil {
		t.root = newRoot
	}
	if !didUpdate {
		t.size++
	}
	return oldVal, didUpdate
}

// Delete is used to delete a given key. Returns the old value if any,
// and a bool indicating if the key was set.
func (t *Txn) Delete(k []byte) (interface{}, bool) {
	newRoot, leaf := t.delete(nil, t.root, k)
	if newRoot != nil {
		t.root = newRoot
	}
	if leaf != nil {
		t.size--
		return leaf.val, true
	}
	return nil, false
}

// Root returns the current root of the radix tree within this
// transaction. The root is not safe across insert and delete operations,
// but can be used to read the current state during a transaction.
func (t *Txn) Root() *Node {
	return t.root
}

// Get is used to lookup a specific key, returning
// the value and if it was found
func (t *Txn) Get(k []byte) (interface{}, bool) {
	return t.root.Get(k)
}

// GetWatch is used to lookup a specific key, returning
// the watch channel, value and if it was found
func (t *Txn) GetWatch(k []byte) (<-chan struct{}, interface{}, bool) {
	return t.root.GetWatch(k)
}

// Commit is used to finalize the transaction and return a new tree. If mutation
// tracking is turned on then notifications will also be issued.
func (t *Txn) Commit() *Tree {
	nt := t.commit()
	if t.trackMutate {
		t.notify()
	}
	return nt
}

// commit is an internal helper for Commit(), useful for unit tests.
func (t *Txn) commit() *Tree {
	nt := &Tree{t.root, t.size}
	t.writable = nil
	return nt
}

// slowNotify does a complete comparison of the before and after trees in order
// to trigger notifications. This doesn't require any additional state but it
// is very expensive to compute.
func (t *Txn) slowNotify() {
	snapIter := t.snap.rawIterator()
	rootIter := t.root.rawIterator()
	for snapIter.Front() != nil || rootIter.Front() != nil {
		// If we've exhausted the nodes in the old snapshot, we know
		// there's nothing remaining to notify.
		if snapIter.Front() == nil {
			return
		}
		snapElem := snapIter.Front()

		// If we've exhausted the nodes in the new root, we know we need
		// to invalidate everything that remains in the old snapshot. We
		// know from the loop condition there's something in the old
		// snapshot.
		if rootIter.Front() == nil {
			close(snapElem.mutateCh)
			if snapElem.isLeaf() {
				close(snapElem.leaf.mutateCh)
			}
			snapIter.Next()
			continue
		}

		// Do one string compare so we can check the various conditions
		// below without repeating the compare.
		cmp := strings.Compare(snapIter.Path(), rootIter.Path())

		// If the snapshot is behind the root, then we must have deleted
		// this node during the transaction.
		if cmp < 0 {
			close(snapElem.mutateCh)
			if snapElem.isLeaf() {
				close(snapElem.leaf.mutateCh)
			}
			snapIter.Next()
			continue
		}

		// If the snapshot is ahead of the root, then we must have added
		// this node during the transaction.
		if cmp > 0 {
			rootIter.Next()
			continue
		}

		// If we have the same path, then we need to see if we mutated a
		// node and possibly the leaf.
		rootElem := rootIter.Front()
		if snapElem != rootElem {
			close(snapElem.mutateCh)
			if snapElem.leaf != nil && (snapElem.leaf != rootElem.leaf) {
				close(snapElem.leaf.mutateCh)
			}
		}
		snapIter.Next()
		rootIter.Next()
	}
}

// notify is used along with TrackMutate to trigger notifications. This should
// only be done once a transaction is committed.
func (t *Txn) notify() {
	// If we've overflowed the tracking state we can't use it in any way and
	// need to do a full tree compare.
	if t.trackOverflow {
		t.slowNotify()
	} else {
		for ch := range t.trackChannels {
			close(*ch)
		}
	}

	// Clean up the tracking state so that a re-notify is safe (will trigger
	// the else clause above which will be a no-op).
	t.trackChannels = nil
	t.trackOverflow = false
}

// Insert is used to add or update a given key. The return provides
// the new tree, previous value and a bool indicating if any was set.
func (t *Tree) Insert(k []byte, v interface{}) (*Tree, interface{}, bool) {
	txn := t.Txn()
	old, ok := txn.Insert(k, v)
	return txn.Commit(), old, ok
}

// Delete is used to delete a given key. Returns the new tree,
// old value if any, and a bool indicating if the key was set.
func (t *Tree) Delete(k []byte) (*Tree, interface{}, bool) {
	txn := t.Txn()
	old, ok := txn.Delete(k)
	return txn.Commit(), old, ok
}

// Root returns the root node of the tree which can be used for richer
// query operations.
func (t *Tree) Root() *Node {
	return t.root
}

// Get is used to lookup a specific key, returning
// the value and if it was found
func (t *Tree) Get(k []byte) (interface{}, bool) {
	return t.root.Get(k)
}

// longestPrefix finds the length of the shared prefix
// of two strings
func longestPrefix(k1, k2 []byte) int {
	max := len(k1)
	if l := len(k2); l < max {
		max = l
	}
	var i int
	for i = 0; i < max; i++ {
		if k1[i] != k2[i] {
			break
		}
	}
	return i
}

// concat two byte slices, returning a third new copy
func concat(a, b []byte) []byte {
	c := make([]byte, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)
	return c
}
