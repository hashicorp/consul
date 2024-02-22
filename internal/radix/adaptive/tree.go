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
	root *Node[T]
	size uint64
}

func NewRadixTree[T any]() *RadixTree[T] {
	rt := &RadixTree[T]{size: 0}
	nodeLeaf := rt.allocNode(leafType)
	rt.root = &nodeLeaf
	return rt
}

func (t *RadixTree[T]) GetPathIterator(path []byte) *PathIterator[T] {
	nodeT := *t.root
	return nodeT.PathIterator(path)
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
		nodeLeaf := t.allocNode(leafType)
		t.root = &nodeLeaf
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
	var child **Node[T]
	n := *t.root
	depth := 0

	for n != nil {
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
		child = t.findChild(n, key[depth])
		if child != nil && *child != nil && **child != nil {
			n = **child
		} else {
			n = nil
		}
		depth++
	}
	return zero, false
}

func (t *RadixTree[T]) recursiveInsert(n *Node[T], ref **Node[T], key []byte, value T, depth int, old *int) T {
	var zero T
	keyLen := len(key)

	// If we are at a nil node, inject a leaf
	if n == nil {
		leafNode := t.makeLeaf(key, value)
		*ref = &leafNode
		return zero
	}

	node := *n
	if node.isLeaf() {
		nodeLeaf := node.(*NodeLeaf[T])
		// This means root is nil
		if len(nodeLeaf.key) == 0 {
			leafNode := t.makeLeaf(key, value)
			*ref = &leafNode
			return zero
		}
	}

	// If we are at a leaf, we need to replace it with a node
	if node.isLeaf() {
		nodeLeaf := node.(*NodeLeaf[T])

		// Check if we are updating an existing value
		if bytes.Equal(nodeLeaf.key, key) {
			*old = 1
			oldVal := nodeLeaf.value
			nodeLeaf.value = value
			return oldVal
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
		t.addChild4(newNode4, ref, nodeLeaf.key[depth+longestPrefix], nodeLeaf)
		t.addChild4(newNode4, ref, newLeaf2.key[depth+longestPrefix], newLeaf2)
		*ref = &newNode
		return zero
	}

	// Check if given node has a prefix
	if node.getPartialLen() > 0 {
		// Determine if the prefixes differ, since we need to split
		prefixDiff := prefixMismatch[T](node, key, keyLen, depth)
		if prefixDiff >= int(node.getPartialLen()) {
			depth += int(node.getPartialLen())
			child := t.findChild(node, key[depth])
			if child != nil {
				return t.recursiveInsert(*child, child, key, value, depth+1, old)
			}

			// No child, node goes within us
			newLeaf := t.makeLeaf(key, value)
			t.addChild(node, ref, key[depth], newLeaf)
			return zero
		}

		// Create a new node
		newNode := t.allocNode(node4)
		*ref = &newNode
		newNode4 := newNode.(*Node4[T])
		newNode4.partialLen = uint32(prefixDiff)
		copy(newNode4.partial[:], node.getPartial()[:min(maxPrefixLen, prefixDiff)])

		// Adjust the prefix of the old node
		if node.getPartialLen() <= maxPrefixLen {
			t.addChild4(newNode4, ref, node.getPartial()[prefixDiff], node)
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			length := min(maxPrefixLen, int(node.getPartialLen()))
			copy(node.getPartial()[:], node.getPartial()[prefixDiff+1:+prefixDiff+1+length])
		} else {
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			l := minimum[T](&node)
			if l == nil {
				return zero
			}
			t.addChild4(newNode4, ref, l.key[depth+prefixDiff], node)
			length := min(maxPrefixLen, int(node.getPartialLen()))
			copy(node.getPartial()[:], l.key[depth+prefixDiff+1:depth+prefixDiff+1+length])
		}
		// Insert the new leaf
		newLeaf := t.makeLeaf(key, value)
		t.addChild4(newNode4, ref, key[depth+prefixDiff], newLeaf)
		return zero
	}
	// Find a child to recurse to
	child := t.findChild(node, key[depth])
	if child != nil {
		return t.recursiveInsert(*child, child, key, value, depth+1, old)
	}

	// No child, node goes within us
	newLeaf := t.makeLeaf(key, value)
	t.addChild(node, ref, key[depth], newLeaf)
	return zero
}

func (t *RadixTree[T]) recursiveDelete(n *Node[T], ref **Node[T], key []byte, depth int) *NodeLeaf[T] {
	keyLen := len(key)
	// Search terminated
	if n == nil {
		return nil
	}
	node := *n
	// Handle hitting a leaf node
	if isLeaf[T](node) {
		l := node.(*NodeLeaf[T])
		if leafMatches[T](l, key, keyLen) == 0 {
			*ref = nil
			return l
		}
		return nil
	}

	// Bail if the prefix does not match
	if node.getPartialLen() > 0 {
		prefixLen := checkPrefix[T](node, key, keyLen, depth)
		if prefixLen != min(maxPrefixLen, int(node.getPartialLen())) {
			return nil
		}
		depth += int(node.getPartialLen())
	}

	// Find child node
	child := t.findChild(node, key[depth])
	if child == nil {
		return nil
	}

	// If the child is a leaf, delete from this node
	if isLeaf[T](**child) {
		nodeChild := **child
		l := nodeChild.(*NodeLeaf[T])
		if leafMatches[T](l, key, keyLen) == 0 {
			t.removeChild(node, ref, key[depth], child)
			return l
		}
		return nil
	}

	// Recurse
	return t.recursiveDelete(*child, child, key, depth+1)
}
