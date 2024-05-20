// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"bytes"
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
}

func NewRadixTree[T any]() *RadixTree[T] {
	rt := &RadixTree[T]{size: 0}
	nodeLeaf := rt.allocNode(leafType)
	rt.root = nodeLeaf
	return rt
}

// Len is used to return the number of elements in the tree
func (t *RadixTree[T]) Len() int {
	return int(t.size)
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

func (t *RadixTree[T]) Get(key []byte) (T, bool, <-chan struct{}) {
	return t.iterativeSearch(getTreeKey(key))
}

func (t *RadixTree[T]) LongestPrefix(k []byte) ([]byte, T, bool) {
	key := getTreeKey(k)
	var zero T
	if t.root == nil {
		return nil, zero, false
	}

	var child, last Node[T]
	depth := 0

	n := t.root
	for n != nil {

		// Bail if the prefix does not match
		if n.getPartialLen() > 0 {
			prefixLen := checkPrefix(n.getPartial(), int(n.getPartialLen()), key, depth)
			if prefixLen != min(maxPrefixLen, int(n.getPartialLen())) {
				break
			}
			depth += int(n.getPartialLen())
		}

		if depth >= len(key) {
			break
		}

		for _, ch := range n.getChildren() {
			if ch != nil {
				if isLeaf[T](ch) && bytes.HasPrefix(getKey(key), getKey(ch.getKey())) {
					last = ch
				}
			}
		}

		// Recursively search
		child, _ = t.findChild(n, key[depth])
		if child == nil {
			break
		}
		n = child
		depth++
	}

	if last != nil {
		return getKey(last.getKey()), last.getValue(), true
	}

	return nil, zero, false
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
	if newRoot == nil {
		newRoot = t.allocNode(leafType)
	}
	t.root = newRoot
	if l != nil {
		t.size--
		old := l.getValue()
		return old
	}
	return zero
}

func (t *RadixTree[T]) iterativeSearch(key []byte) (T, bool, <-chan struct{}) {
	var zero T
	if t.root == nil {
		return zero, false, nil
	}
	var child Node[T]
	depth := 0

	n := t.root
	for n != nil {
		// Might be a leaf
		if isLeaf[T](n) {
			// Check if the expanded path matches
			if leafMatches(n.getKey(), key) == 0 {
				return n.getValue(), true, n.getMutateCh()
			}
			return zero, false, nil
		}

		// Bail if the prefix does not match
		if n.getPartialLen() > 0 {
			prefixLen := checkPrefix(n.getPartial(), int(n.getPartialLen()), key, depth)
			if prefixLen != min(maxPrefixLen, int(n.getPartialLen())) {
				return zero, false, nil
			}
			depth += int(n.getPartialLen())
		}

		if depth >= len(key) {
			return zero, false, nil
		}

		// Recursively search
		child, _ = t.findChild(n, key[depth])
		if child == nil {
			return zero, false, nil
		}
		n = child
		depth++
	}
	return zero, false, nil
}

func (t *RadixTree[T]) recursiveInsert(node Node[T], key []byte, value T, depth int, old *int) (Node[T], T) {
	var zero T

	// If we are at a nil node, inject a leaf
	if node == nil {
		return t.makeLeaf(key, value), zero
	}

	if node.isLeaf() {
		// This means node is nil
		if node.getKeyLen() == 0 {
			return t.makeLeaf(key, value), zero
		}
	}

	// If we are at a leaf, we need to replace it with a node
	if node.isLeaf() {
		// Check if we are updating an existing value
		nodeKey := node.getKey()
		if len(key) == len(nodeKey) && bytes.Equal(nodeKey, key) {
			*old = 1
			return t.makeLeaf(key, value), node.getValue()
		}

		// New value, we must split the leaf into a node4
		newLeaf2 := t.makeLeaf(key, value)

		// Determine longest prefix
		longestPrefix := longestCommonPrefix[T](node, newLeaf2, depth)
		newNode := t.allocNode(node4)
		newNode.setPartialLen(uint32(longestPrefix))
		copy(newNode.getPartial()[:], key[depth:depth+min(maxPrefixLen, longestPrefix)])

		// Add the leafs to the new node4
		newNode = t.addChild(newNode, node.getKey()[depth+longestPrefix], node)
		newNode = t.addChild(newNode, newLeaf2.getKey()[depth+longestPrefix], newLeaf2)
		return newNode, zero
	}

	// Check if given node has a prefix
	if node.getPartialLen() > 0 {
		// Determine if the prefixes differ, since we need to split
		prefixDiff := prefixMismatch[T](node, key, len(key), depth)
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
		newNode.setPartialLen(uint32(prefixDiff))
		copy(newNode.getPartial()[:], node.getPartial()[:min(maxPrefixLen, prefixDiff)])

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
	return t.addChild(node, key[depth], newLeaf), zero
}

func (t *RadixTree[T]) recursiveDelete(node Node[T], key []byte, depth int) (Node[T], Node[T]) {
	// Get terminated
	if node == nil {
		return nil, nil
	}
	// Handle hitting a leaf node
	if isLeaf[T](node) {
		if leafMatches(node.getKey(), key) == 0 {
			return nil, node
		}
		return node, nil
	}

	// Bail if the prefix does not match
	if node.getPartialLen() > 0 {
		prefixLen := checkPrefix(node.getPartial(), int(node.getPartialLen()), key, depth)
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
		if leafMatches(child.getKey(), key) == 0 {
			return t.removeChild(node.clone(), key[depth]), child
		}
		return node, nil
	}

	// Recurse
	newChild, val := t.recursiveDelete(child.clone(), key, depth+1)
	nodeClone := node.clone()
	nodeClone.setChild(idx, newChild)
	return nodeClone, val
}
