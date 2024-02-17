// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import "bytes"

type Node16[T any] struct {
	partialLen  uint32
	artNodeType uint8
	numChildren uint8
	partial     []byte
	keys        [16]byte
	children    [16]*Node[T]
}

func (n *Node16[T]) getPartialLen() uint32 {
	return n.partialLen
}

func (n *Node16[T]) setPartialLen(partialLen uint32) {
	n.partialLen = partialLen
}

func (n *Node16[T]) getArtNodeType() uint8 {
	return n.artNodeType
}

func (n *Node16[T]) setArtNodeType(artNodeType uint8) {
	n.artNodeType = artNodeType
}

func (n *Node16[T]) getNumChildren() uint8 {
	return n.numChildren
}

func (n *Node16[T]) setNumChildren(numChildren uint8) {
	n.numChildren = numChildren
}

func (n *Node16[T]) getPartial() []byte {
	return n.partial
}

func (n *Node16[T]) setPartial(partial []byte) {
	n.partial = partial
}

func (n *Node16[T]) isLeaf() bool {
	return false
}

// Iterator is used to return an iterator at
// the given node to walk the tree
func (n *Node16[T]) Iterator() *Iterator[T] {
	stack := make([]Node[T], 0)
	stack = append(stack, n)
	return &Iterator[T]{stack: stack, root: n}
}

func (n *Node16[T]) PathIterator(path []byte) *PathIterator[T] {
	return &PathIterator[T]{parent: n, path: path}
}

func (n *Node16[T]) matchPrefix(prefix []byte) bool {
	return bytes.HasPrefix(n.partial, prefix)
}

func (n *Node16[T]) getChild(index int) *Node[T] {
	return n.children[index]
}
