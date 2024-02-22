// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"sync"
)

const maxPrefixLen = 10
type nodeType int
const (
   leafType nodeType = iota
   node4
   node16
   node48
   node256
)

type RadixTree[T any] struct {
	root *Node[T]
	size uint64
	mu   *sync.RWMutex
}

func (t *RadixTree[T]) GetPathIterator(path []byte) *PathIterator[T] {
	nodeT := *t.root
	nodeT.setMutex(t.mu)
	return nodeT.PathIterator(path)
}

func NewRadixTree[T any]() *RadixTree[T] {
	rt := &RadixTree[T]{size: 0, mu: &sync.RWMutex{}}
	nodeLeaf := rt.allocNode(LEAF)
	rt.root = &nodeLeaf
	return rt
}

func (t *RadixTree[T]) Insert(key []byte, value T) T {
	var old int
	oldVal := t.recursiveInsert(t.root, &t.root, getTreeKey(key), value, 0, &old)
	if old == 0 {
		t.size++
	}
	return oldVal
}

func (t *RadixTree[T]) Search(key []byte) (T, bool) {
	val, found := t.iterativeSearch(getTreeKey(key))
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
	l := t.recursiveDelete(t.root, &t.root, getTreeKey(key), 0)
	if t.root == nil {
		nodeLeaf := t.allocNode(LEAF)
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
	root *Node[T]

	size uint64

	tree *RadixTree[T]
}

// Txn starts a new transaction that can be used to mutate the tree
func (t *RadixTree[T]) Txn() *Txn[T] {
	txn := &Txn[T]{
		root: t.root,
		size: t.size,
		tree: t,
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
	t.tree.mu.Lock()
	defer t.tree.mu.Unlock()
	oldVal := t.tree.Insert(key, value)
	t.root = t.tree.root
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Delete(key []byte) T {
	t.tree.mu.Lock()
	defer t.tree.mu.Unlock()
	oldVal := t.tree.Delete(key)
	t.root = t.tree.root
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Root() Node[T] {
	return *t.root
}

func (t *Txn[T]) Commit() *RadixTree[T] {
	return t.tree
}
