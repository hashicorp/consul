// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"bytes"
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
	root Node[T]
	size uint64
	mu   *sync.RWMutex
}

func NewRadixTree[T any]() *RadixTree[T] {
	rt := &RadixTree[T]{size: 0, mu: &sync.RWMutex{}}
	nodeLeaf := rt.allocNode(leafType)
	rt.root = nodeLeaf
	return rt
}

func (t *RadixTree[T]) GetPathIterator(path []byte) *PathIterator[T] {
	nodeT := t.root
	return nodeT.PathIterator(path)
}

func (t *RadixTree[T]) Insert(key []byte, value T) T {
	var old int
	newRoot, oldVal := t.recursiveInsert(t.root, getTreeKey(key), value, 0, &old)
	if old == 0 {
		t.size++
	}
	t.root = newRoot
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
	newRoot, l := t.recursiveDelete(t.root, getTreeKey(key), 0)
	t.root = newRoot
	if t.root == nil {
		nodeLeaf := t.allocNode(leafType)
		t.root = nodeLeaf
	}
	if l != nil {
		t.size--
		old := l.value
		return old
	}
	return zero
}

func (t *RadixTree[T]) iterativeSearch(key []byte) (T, bool) {
	var zero T
	if t.root == nil {
		return zero, false
	}
	keyLen := len(key)
	var child Node[T]
	depth := 0

	n := t.root
	for t.root != nil {
		// Might be a leaf
		if isLeaf[T](n) {
			leaf, ok := n.(*NodeLeaf[T])
			if !ok {
				continue
			}
			// Check if the expanded path matches
			if leafMatches[T](leaf, key, keyLen) == 0 {
				return leaf.value, true
			}
			return zero, false
		}

		// Bail if the prefix does not match
		if n.getPartialLen() > 0 {
			prefixLen := checkPrefix[T](n, key, keyLen, depth)
			if prefixLen != min(maxPrefixLen, int(n.getPartialLen())) {
				return zero, false
			}
			depth += int(n.getPartialLen())
		}

		if depth >= keyLen {
			return zero, false
		}

		// Recursively search
		child, _ = t.findChild(n, key[depth])
		if child != nil {
			n = child
		} else {
			n = nil
		}
		depth++
	}
	return zero, false
}

func (t *RadixTree[T]) recursiveInsert(n Node[T], key []byte, value T, depth int, old *int) (Node[T], T) {
	var zero T
	keyLen := len(key)

	// If we are at a nil node, inject a leaf
	if n == nil {
		leafNode := t.makeLeaf(key, value)
		return leafNode, zero
	}

	node := n
	if node.isLeaf() {
		nodeLeaf := node.(*NodeLeaf[T])
		// This means root is nil
		if len(nodeLeaf.key) == 0 {
			leafNode := t.makeLeaf(key, value)
			return leafNode, zero
		}
	}

	// If we are at a leaf, we need to replace it with a node
	if node.isLeaf() {
		nodeLeaf := node.(*NodeLeaf[T])

		// Check if we are updating an existing value
		if len(key) == len(nodeLeaf.key) && bytes.Equal(nodeLeaf.key, key) {
			*old = 1
			oldVal := nodeLeaf.value
			newNode := t.makeLeaf(key, value)
			return newNode, oldVal
		}

		// New value, we must split the leaf into a node4
		newLeaf2 := t.makeLeaf(key, value).(*NodeLeaf[T])

		// Determine longest prefix
		longestPrefix := longestCommonPrefix[T](nodeLeaf, newLeaf2, depth)
		newNode := t.allocNode(node4)
		newNode4 := newNode.(*Node4[T])
		newNode4.partialLen = uint32(longestPrefix)
		copy(newNode4.partial[:], key[depth:depth+min(maxPrefixLen, longestPrefix)])

		// Add the leafs to the new node4
		newNode = t.addChild(newNode, nodeLeaf.key[depth+longestPrefix], nodeLeaf)
		newNode = t.addChild(newNode, newLeaf2.key[depth+longestPrefix], newLeaf2)
		return newNode, zero
	}

	// Check if given node has a prefix
	if node.getPartialLen() > 0 {
		// Determine if the prefixes differ, since we need to split
		prefixDiff := prefixMismatch[T](node, key, keyLen, depth)
		if prefixDiff >= int(node.getPartialLen()) {
			depth += int(node.getPartialLen())
			child, idx := t.findChild(node, key[depth])
			if child != nil {
				newChild, val := t.recursiveInsert(child, key, value, depth+1, old)
				node.setChild(idx, newChild)
				return node, val
			}

			// No child, node goes within us
			newLeaf := t.makeLeaf(key, value)
			node = t.addChild(node, key[depth], newLeaf)
			return node, zero
		}

		// Create a new node
		newNode := t.allocNode(node4)
		newNode4 := newNode.(*Node4[T])
		newNode4.partialLen = uint32(prefixDiff)
		copy(newNode4.partial[:], node.getPartial()[:min(maxPrefixLen, prefixDiff)])

		// Adjust the prefix of the old node
		if node.getPartialLen() <= maxPrefixLen {
			newNode = t.addChild(newNode, node.getPartial()[prefixDiff], node)
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			length := min(maxPrefixLen, int(node.getPartialLen()))
			copy(node.getPartial()[:], node.getPartial()[prefixDiff+1:+prefixDiff+1+length])
		} else {
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			l := minimum[T](node)
			if l == nil {
				return node, zero
			}
			newNode = t.addChild(newNode, l.key[depth+prefixDiff], node)
			length := min(maxPrefixLen, int(node.getPartialLen()))
			copy(node.getPartial()[:], l.key[depth+prefixDiff+1:depth+prefixDiff+1+length])
		}
		// Insert the new leaf
		newLeaf := t.makeLeaf(key, value)
		newNode = t.addChild(newNode, key[depth+prefixDiff], newLeaf)
		return newNode, zero
	}
	// Find a child to recurse to
	child, idx := t.findChild(node, key[depth])
	if child != nil {
		newChild, val := t.recursiveInsert(child, key, value, depth+1, old)
		node.setChild(idx, newChild)
		return node, val
	}

	// No child, node goes within us
	newLeaf := t.makeLeaf(key, value)
	node = t.addChild(node, key[depth], newLeaf)
	return node, zero
}

func (t *RadixTree[T]) recursiveDelete(n Node[T], key []byte, depth int) (Node[T], *NodeLeaf[T]) {
	keyLen := len(key)
	// Search terminated
	if n == nil {
		return nil, nil
	}
	node := n
	// Handle hitting a leaf node
	if isLeaf[T](node) {
		l := node.(*NodeLeaf[T])
		if leafMatches[T](l, key, keyLen) == 0 {
			return nil, l
		}
		return n, nil
	}

	// Bail if the prefix does not match
	if node.getPartialLen() > 0 {
		prefixLen := checkPrefix[T](node, key, keyLen, depth)
		if prefixLen != min(maxPrefixLen, int(node.getPartialLen())) {
			return node, nil
		}
		depth += int(node.getPartialLen())
	}

	// Find child node
	child, idx := t.findChild(node, key[depth])
	if child == nil {
		return nil, nil
	}

	// If the child is a leaf, delete from this node
	if isLeaf[T](child) {
		nodeChild := child
		l := nodeChild.(*NodeLeaf[T])
		if leafMatches[T](l, key, keyLen) == 0 {
			node = t.removeChild(node.Clone(), key[depth], &child)
			return node, l
		}
		return node, nil
	}

	// Recurse
	newChild, val := t.recursiveDelete(child, key, depth+1)
	newNode := node.Clone()
	newNode.setChild(idx, newChild)
	return newNode, val
}
