// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

type Node256[T any] struct {
	partialLen  uint32
	artNodeType uint8
	numChildren uint8
	partial     []byte
	children    [256]*Node[T]
}

func (n *Node256[T]) getPartialLen() uint32 {
	return n.partialLen
}

func (n *Node256[T]) setPartialLen(partialLen uint32) {
	n.partialLen = partialLen
}

func (n *Node256[T]) getArtNodeType() uint8 {
	return n.artNodeType
}

func (n *Node256[T]) setArtNodeType(artNodeType uint8) {
	n.artNodeType = artNodeType
}

func (n *Node256[T]) getNumChildren() uint8 {
	return n.numChildren
}

func (n *Node256[T]) setNumChildren(numChildren uint8) {
	n.numChildren = numChildren
}

func (n *Node256[T]) getPartial() []byte {
	return n.partial
}

func (n *Node256[T]) setPartial(partial []byte) {
	n.partial = partial
}

func (n *Node256[T]) isLeaf() bool {
	return false
}
