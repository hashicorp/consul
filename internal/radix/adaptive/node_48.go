// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import "bytes"

type Node48[T any] struct {
	partialLen  uint32
	artNodeType uint8
	numChildren uint8
	partial     []byte
	keys        [256]byte
	children    [48]*Node[T]
}

func (n *Node48[T]) getPartialLen() uint32 {
	return n.partialLen
}

func (n *Node48[T]) setPartialLen(partialLen uint32) {
	n.partialLen = partialLen
}

func (n *Node48[T]) getArtNodeType() uint8 {
	return n.artNodeType
}

func (n *Node48[T]) setArtNodeType(artNodeType uint8) {
	n.artNodeType = artNodeType
}

func (n *Node48[T]) getNumChildren() uint8 {
	return n.numChildren
}

func (n *Node48[T]) setNumChildren(numChildren uint8) {
	n.numChildren = numChildren
}

func (n *Node48[T]) getPartial() []byte {
	return n.partial
}

func (n *Node48[T]) setPartial(partial []byte) {
	n.partial = partial
}

func (n *Node48[T]) isLeaf() bool {
	return false
}

// Iterator is used to return an iterator at
// the given node to walk the tree
func (n *Node48[T]) Iterator() *Iterator[T] {
	stack := make([]Node[T], 0)
	stack = append(stack, n)
	nodeT := Node[T](n)
	return &Iterator[T]{stack: stack, root: &nodeT}
}

func (n *Node48[T]) PathIterator(path []byte) *PathIterator[T] {
	return &PathIterator[T]{parent: n, path: path}
}

func (n *Node48[T]) matchPrefix(prefix []byte) bool {
	for i := 0; i < 256; i++ {
		if n.keys[i] == 0 {
			continue
		}
		childPrefix := []byte{byte(i)}
		if bytes.HasPrefix(childPrefix, prefix) {
			return true
		}
	}
	return false
}

func (n *Node48[T]) getChild(index int) *Node[T] {
	return n.children[index]
}
