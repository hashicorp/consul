// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import "bytes"

type NodeLeaf[T any] struct {
	value       T
	keyLen      uint32
	key         []byte
	artNodeType uint8
}

func (n *NodeLeaf[T]) getPartialLen() uint32 {
	// no-op
	return 0
}

func (n *NodeLeaf[T]) setPartialLen(partialLen uint32) {
	// no-op
}

func (n *NodeLeaf[T]) getArtNodeType() uint8 {
	return n.artNodeType
}

func (n *NodeLeaf[T]) setArtNodeType(artNodeType uint8) {
	n.artNodeType = artNodeType
}

func (n *NodeLeaf[T]) getNumChildren() uint8 {
	return 0
}

func (n *NodeLeaf[T]) setNumChildren(numChildren uint8) {
	// no-op
}

func (n *NodeLeaf[T]) isLeaf() bool {
	return true
}

func (n *NodeLeaf[T]) getValue() interface{} {
	return n.value
}

func (n *NodeLeaf[T]) setValue(value T) {
	n.value = value
}

func (n *NodeLeaf[T]) getKeyLen() uint32 {
	return n.keyLen
}

func (n *NodeLeaf[T]) setKeyLen(keyLen uint32) {
	n.keyLen = keyLen
}

func (n *NodeLeaf[T]) getKey() []byte {
	return n.key
}

func (n *NodeLeaf[T]) setKey(key []byte) {
	n.key = key
}

func (n *NodeLeaf[T]) getPartial() []byte {
	//no-op
	return []byte{}
}

func (n *NodeLeaf[T]) setPartial(partial []byte) {
	// no-op
}

func (l *NodeLeaf[T]) prefixContainsMatch(key []byte) bool {
	if len(key) == 0 || len(l.key) == 0 {
		return false
	}
	if key == nil || len(l.key)-1 > len(key) {
		return false
	}

	return bytes.Compare(key[:len(l.key)-1], l.key[:len(l.key)-1]) == 0
}

func (l *NodeLeaf[T]) prefixMatch(key []byte) bool {
	if key == nil || len(l.key)-1 > len(key) {
		return false
	}

	return bytes.Compare(key[:len(l.key)-1], l.key[:len(l.key)-1]) == 0
}

func (n *NodeLeaf[T]) Iterator() *Iterator[T] {
	stack := make([]Node[T], 0)
	stack = append(stack, n)
	nodeT := Node[T](n)
	return &Iterator[T]{stack: stack, root: &nodeT}
}

func (n *NodeLeaf[T]) PathIterator(path []byte) *PathIterator[T] {
	nodeT := Node[T](n)
	return &PathIterator[T]{parent: &nodeT, path: path}
}

func (n *NodeLeaf[T]) matchPrefix(prefix []byte) bool {
	if len(n.key) == 0 {
		return false
	}
	actualKey := n.key[:len(n.key)-1]
	actualPrefix := prefix[:len(prefix)-1]
	return bytes.HasPrefix(actualKey, actualPrefix)
}

func (n *NodeLeaf[T]) getChild(index int) *Node[T] {
	return nil
}
