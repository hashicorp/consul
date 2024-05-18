// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"bytes"
)

type Node16[T any] struct {
	partialLen  uint32
	numChildren uint8
	partial     []byte
	keys        [16]byte
	children    [16]Node[T]
}

func (n *Node16[T]) getPartialLen() uint32 {
	return n.partialLen
}

func (n *Node16[T]) setPartialLen(partialLen uint32) {
	n.partialLen = partialLen
}

func (n *Node16[T]) getArtNodeType() nodeType {
	return node16
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
	nodeT := Node[T](n)
	return &Iterator[T]{
		stack: stack,
		root:  &nodeT,
	}
}

func (n *Node16[T]) PathIterator(path []byte) *PathIterator[T] {
	nodeT := Node[T](n)
	return &PathIterator[T]{
		parent: &nodeT,
		path:   getTreeKey(path),
		stack:  []Node[T]{nodeT},
	}
}

func (n *Node16[T]) matchPrefix(prefix []byte) bool {
	return bytes.HasPrefix(n.partial, prefix)
}

func (n *Node16[T]) getChild(index int) Node[T] {
	return n.children[index]
}

func (n *Node16[T]) Clone() Node[T] {
	newNode := &Node16[T]{
		partialLen:  n.getPartialLen(),
		numChildren: n.getNumChildren(),
		partial:     n.getPartial(),
	}
	copy(newNode.keys[:], n.keys[:])
	copy(newNode.children[:], n.children[:])
	nodeT := Node[T](newNode)
	return nodeT
}

func (n *Node16[T]) getKeyLen() uint32 {
	return 0
}

func (n *Node16[T]) setKeyLen(keyLen uint32) {

}

func (n *Node16[T]) setChild(index int, child Node[T]) {
	n.children[index] = child
}
func (n *Node16[T]) getKey() []byte {
	//no op
	return []byte{}
}

func (n *Node16[T]) getValue() T {
	//no op
	var zero T
	return zero
}

func (n *Node16[T]) getKeyAtIdx(idx int) byte {
	return n.keys[idx]
}

func (n *Node16[T]) setKeyAtIdx(idx int, key byte) {
	n.keys[idx] = key
}

func (n *Node16[T]) getChildren() []Node[T] {
	return n.children[:]
}

func (n *Node16[T]) getKeys() []byte {
	return n.keys[:]
}
