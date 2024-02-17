// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"github.com/hashicorp/golang-lru/v2/simplelru"
	"sync"
)

const MaxPrefixLen = 1000
const LEAF = 0
const NODE4 = 1
const NODE16 = 2
const NODE48 = 3
const NODE256 = 4

type RadixTree[T any] struct {
	root *Node[T]
	size uint64
}

func (t *RadixTree[T]) GetPathIterator(path []byte) *PathIterator[T] {
	return &PathIterator[T]{parent: t.root, path: getTreeKey(path)}
}

func NewAdaptiveRadixTree[T any]() *RadixTree[T] {
	nodeLeaf := allocNode[T](LEAF)
	return &RadixTree[T]{root: &nodeLeaf, size: 0}
}

func (t *RadixTree[T]) Insert(key []byte, value T) T {
	var old int
	oldVal := recursiveInsert[T](t.root, &t.root, getTreeKey(key), value, 0, &old)
	if old == 0 {
		t.size++
	}
	return oldVal
}

func (t *RadixTree[T]) Search(key []byte) (T, bool) {
	val, found := iterativeSearch[T](t, getTreeKey(key))
	return val, found
}

func (t *RadixTree[T]) Minimum() *NodeLeaf[T] {
	return minimum[T](t.root)
}

func (t *RadixTree[T]) Maximum() *NodeLeaf[T] {
	return maximum[T](t.root)
}

func (t *RadixTree[T]) Delete(key []byte) T {
	var zero T
	l := recursiveDelete[T](t.root, &t.root, getTreeKey(key), 0)
	if t.root == nil {
		nodeLeaf := allocNode[T](LEAF)
		t.root = &nodeLeaf
	}
	if l != nil {
		t.size--
		old := l.value
		return old
	}
	return zero
}

type Txn[T any] struct {
	// root is the modified root for the transaction.
	root *Node[T]

	// snap is a snapshot of the root node for use if we have to run the
	// slow notify algorithm.
	snap *Node[T]

	// size tracks the size of the tree as it is modified during the
	// transaction.
	size uint64

	// writable is a cache of writable nodes that have been created during
	// the course of the transaction. This allows us to re-use the same
	// nodes for further writes and avoid unnecessary copies of nodes that
	// have never been exposed outside the transaction. This will only hold
	// up to defaultModifiedCache number of entries.
	writable *simplelru.LRU[*Node[T], any]

	tree *RadixTree[T]

	mutex *sync.RWMutex
}

// Txn starts a new transaction that can be used to mutate the tree
func (t *RadixTree[T]) Txn() *Txn[T] {
	txn := &Txn[T]{
		root:  t.root,
		snap:  t.root,
		size:  t.size,
		tree:  t,
		mutex: &sync.RWMutex{},
	}
	return txn
}

// Get is used to lookup a specific key, returning
// the value and if it was found
func (t *Txn[T]) Get(k []byte) (T, bool) {
	res, found := t.tree.Search(k)
	return res, found
}

func (t *Txn[T]) Insert(key []byte, value T) T {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	oldVal := t.tree.Insert(key, value)
	t.root = t.tree.root
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Delete(key []byte) T {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	oldVal := t.tree.Delete(key)
	t.root = t.tree.root
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Root() *Node[T] {
	return t.root
}
