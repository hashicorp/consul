// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

type Txn[T any] struct {
	root *Node[T]

	size uint64

	tree *RadixTree[T]
}

// Txn starts a new transaction that can be used to mutate the tree
func (t *RadixTree[T]) Txn() *Txn[T] {
	txn := &Txn[T]{
		root: &t.root,
		size: t.size,
		tree: t,
	}
	return txn
}

// Get is used to look up a specific key, returning
// the value and if it was found
func (t *Txn[T]) Get(k []byte) (T, bool) {
	res, found := t.tree.Search(k)
	return res, found
}

func (t *Txn[T]) Insert(key []byte, value T) T {
	oldVal := t.tree.Insert(key, value)
	t.root = &t.tree.root
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Delete(key []byte) T {
	oldVal := t.tree.Delete(key)
	t.root = &t.tree.root
	t.size = t.tree.size
	return oldVal
}

func (t *Txn[T]) Root() Node[T] {
	return *t.root
}

func (t *Txn[T]) Commit() *RadixTree[T] {
	return t.tree
}
