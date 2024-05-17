// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"bytes"
	"sort"
)

func checkPrefix(partial []byte, partialLen int, key []byte, depth int) int {
	maxCmp := min(min(partialLen, maxPrefixLen), len(key)-depth)
	var idx int
	for idx = 0; idx < maxCmp; idx++ {
		if partial[idx] != key[depth+idx] {
			return idx
		}
	}
	return idx
}

func leafMatches(nodeKey []byte, key []byte) int {
	// Fail if the key lengths are different
	if len(nodeKey) != len(key) {
		return 1
	}
	// Compare the keys
	return bytes.Compare(nodeKey, key)
}

func (t *RadixTree[T]) makeLeaf(key []byte, value T) Node[T] {
	// Allocate memory for the leaf node
	l := t.allocNode(leafType).(*NodeLeaf[T])
	if l == nil {
		return nil
	}

	// Set the value and key length
	l.value = value
	l.keyLen = uint32(len(key))
	l.key = make([]byte, l.keyLen)

	// Copy the key
	copy(l.key[:], key)

	return Node[T](l)
}

func (t *RadixTree[T]) allocNode(ntype nodeType) Node[T] {
	var n Node[T]
	switch ntype {
	case leafType:
		n = &NodeLeaf[T]{}
	case node4:
		n = &Node4[T]{}
	case node16:
		n = &Node16[T]{}
	case node48:
		n = &Node48[T]{}
	case node256:
		n = &Node256[T]{}
	default:
		panic("Unknown node type")
	}
	n.setPartial(make([]byte, maxPrefixLen))
	n.setPartialLen(maxPrefixLen)
	return n
}

// longestCommonPrefix finds the length of the longest common prefix between two leaf nodes.
func longestCommonPrefix[T any](l1, l2 Node[T], depth int) int {
	maxCmp := len(l2.getKey()) - depth
	if len(l1.getKey()) < len(l2.getKey()) {
		maxCmp = int(l1.getKeyLen()) - depth
	}
	var idx int
	for idx = 0; idx < maxCmp; idx++ {
		if l1.getKey()[depth+idx] != l2.getKey()[depth+idx] {
			return idx
		}
	}
	return idx
}

// addChild adds a child node to the parent node.
func (t *RadixTree[T]) addChild(n Node[T], c byte, child Node[T]) Node[T] {
	switch n.getArtNodeType() {
	case node4:
		return t.addChild4(n.(*Node4[T]), c, child)
	case node16:
		return t.addChild16(n.(*Node16[T]), c, child)
	case node48:
		return t.addChild48(n.(*Node48[T]), c, child)
	case node256:
		return t.addChild256(n.(*Node256[T]), c, child)
	default:
		panic("Unknown node type")
	}
}

// addChild4 adds a child node to a node4.
func (t *RadixTree[T]) addChild4(n *Node4[T], c byte, child Node[T]) Node[T] {
	if n.numChildren < 4 {
		idx := sort.Search(int(n.numChildren), func(i int) bool {
			return n.keys[i] > c
		})
		// Shift to make room
		length := int(n.numChildren) - idx
		copy(n.keys[idx+1:], n.keys[idx:idx+length])
		copy(n.children[idx+1:], n.children[idx:idx+length])

		// Insert element
		n.keys[idx] = c
		n.children[idx] = child
		n.numChildren++
		return n
	} else {
		newNode := t.allocNode(node16)
		n16 := newNode.(*Node16[T])
		// Copy the child pointers and the key map
		copy(n16.children[:], n.children[:n.numChildren])
		copy(n16.keys[:], n.keys[:n.numChildren])
		t.copyHeader(newNode, n)
		newNode = t.addChild16(n16, c, child)
		return newNode
	}
}

// addChild16 adds a child node to a node16.
func (t *RadixTree[T]) addChild16(n *Node16[T], c byte, child Node[T]) Node[T] {
	if n.numChildren < 16 {
		idx := sort.Search(int(n.numChildren), func(i int) bool {
			return n.keys[i] > c
		})
		// Set the child
		length := int(n.numChildren) - idx
		copy(n.keys[idx+1:], n.keys[idx:idx+length])
		copy(n.children[idx+1:], n.children[idx:idx+length])

		// Insert element
		n.keys[idx] = c
		n.children[idx] = child
		n.numChildren++
		return n
	} else {
		newNode := t.allocNode(node48)
		n48 := newNode.(*Node48[T])
		// Copy the child pointers and populate the key map
		copy(n48.children[:], n.children[:n.numChildren])
		for i := 0; i < int(n.numChildren); i++ {
			n48.keys[n.keys[i]] = byte(i + 1)
		}
		t.copyHeader(newNode, n)
		newNode = t.addChild48(n48, c, child)
		return newNode
	}
}

// addChild48 adds a child node to a node48.
func (t *RadixTree[T]) addChild48(n *Node48[T], c byte, child Node[T]) Node[T] {
	if n.numChildren < 48 {
		pos := 0
		for n.children[pos] != nil {
			pos++
		}
		n.children[pos] = child
		n.keys[c] = byte(pos + 1)
		n.numChildren++
		return n
	} else {
		newNode := t.allocNode(node256)
		n256 := newNode.(*Node256[T])
		for i := 0; i < 256; i++ {
			if n.keys[i] != 0 {
				n256.children[i] = n.children[int(n.keys[i])-1]
			}
		}
		t.copyHeader(newNode, n)
		newNode = t.addChild256(n256, c, child)
		return newNode
	}
}

// addChild256 adds a child node to a node256.
func (t *RadixTree[T]) addChild256(n *Node256[T], c byte, child Node[T]) Node[T] {
	n.numChildren++
	n.children[c] = child
	return Node[T](n)
}

// copyHeader copies header information from src to dest node.
func (t *RadixTree[T]) copyHeader(dest, src Node[T]) {
	dest.setNumChildren(src.getNumChildren())
	dest.setPartialLen(src.getPartialLen())
	length := min(maxPrefixLen, int(src.getPartialLen()))
	copy(dest.getPartial()[:length], src.getPartial()[:length])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// prefixMismatch calculates the index at which the prefixes mismatch.
func prefixMismatch[T any](n Node[T], key []byte, keyLen, depth int) int {
	maxCmp := min(min(maxPrefixLen, int(n.getPartialLen())), keyLen-depth)
	var idx int
	for idx = 0; idx < maxCmp; idx++ {
		if n.getPartial()[idx] != key[depth+idx] {
			return idx
		}
	}

	// If the prefix is short we can avoid finding a leaf
	if n.getPartialLen() > maxPrefixLen {
		// Prefix is longer than what we've checked, find a leaf
		l := minimum(n)
		if l == nil {
			return idx
		}
		maxCmp = min(int(l.keyLen), keyLen) - depth
		for ; idx < maxCmp; idx++ {
			if l.key[idx+depth] != key[depth+idx] {
				return idx
			}
		}
	}
	return idx
}

// minimum finds the minimum leaf under a node.
func minimum[T any](n Node[T]) *NodeLeaf[T] {
	// Handle base cases
	if n == nil {
		return nil
	}
	node := n
	if isLeaf[T](node) {
		return node.(*NodeLeaf[T])
	}

	var idx int
	switch node.getArtNodeType() {
	case node4:
		return minimum[T](node.getChild(0))
	case node16:
		return minimum[T](node.getChild(0))
	case node48:
		idx = 0
		n48 := node.(*Node48[T])
		for idx < 256 && n48.keys[idx] == 0 {
			idx++
		}
		idx = int(n48.keys[idx] - 1)
		if idx < 48 {
			return minimum[T](n48.children[idx])
		}
	case node256:
		n256 := node.(*Node256[T])
		idx = 0
		for idx < 256 && n256.children[idx] == nil {
			idx++
		}
		if idx < 256 {
			return minimum[T](n256.children[idx])
		}
	default:
		panic("Unknown node type")
	}
	return nil
}

// maximum finds the maximum leaf under a node.
func maximum[T any](n Node[T]) *NodeLeaf[T] {
	// Handle base cases
	if n == nil {
		return nil
	}

	node := n

	if isLeaf[T](node) {
		return node.(*NodeLeaf[T])
	}
	var idx int
	switch node.getArtNodeType() {
	case node4:
		return maximum[T](node.(*Node4[T]).children[node.getNumChildren()-1])
	case node16:
		return maximum[T](node.(*Node16[T]).children[node.getNumChildren()-1])
	case node48:
		n48 := node.(*Node48[T])
		idx = 255
		for idx >= 0 && n48.children[idx] == nil {
			idx--
		}
		if idx >= 0 {
			return maximum[T](n48.children[idx])
		}
	case node256:
		idx = 255
		n256 := node.(*Node256[T])
		for idx >= 0 && n256.children[idx] == nil {
			idx--
		}
		if idx >= 0 {
			return maximum[T](n256.children[idx])
		}
	default:
		panic("Unknown node type")
	}
	return nil
}

// IS_LEAF checks whether the least significant bit of the pointer x is set.
func isLeaf[T any](node Node[T]) bool {
	if node == nil {
		return false
	}
	return node.isLeaf()
}

// findChild finds the child node pointer based on the given character in the ART tree node.
func (t *RadixTree[T]) findChild(n Node[T], c byte) (Node[T], int) {
	return findChild(n, c)
}
func findChild[T any](n Node[T], c byte) (Node[T], int) {
	switch n.getArtNodeType() {
	case node4:
		idx := sort.Search(int(n.getNumChildren()), func(i int) bool {
			return n.getKeyAtIdx(i) > c
		})
		if idx >= 1 && n.getKeyAtIdx(idx-1) == c {
			return n.getChild(idx - 1), idx - 1
		}
	case node16:
		// Compare the key to all 16 stored keys
		idx := sort.Search(int(n.getNumChildren()), func(i int) bool {
			return n.getKeyAtIdx(i) > c
		})
		if idx >= 1 && n.getKeyAtIdx(idx-1) == c {
			return n.getChild(idx - 1), idx - 1
		}
	case node48:
		i := n.getKeyAtIdx(int(c))
		if i != 0 {
			return n.getChild(int(i - 1)), int(i - 1)
		}
	case node256:
		if n.getChild(int(c)) != nil {
			return n.getChild(int(c)), int(c)
		}
	case leafType:
		// no-op
		return nil, 0
	default:
		panic("Unknown node type")
	}
	return nil, 0
}

func getTreeKey(key []byte) []byte {
	key = append(key, '$')
	return key
}

func getKey(key []byte) []byte {
	return key[:len(key)-1]
}

func (t *RadixTree[T]) removeChild(n Node[T], c byte) Node[T] {
	switch n.getArtNodeType() {
	case node4:
		return t.removeChild4(n.(*Node4[T]), c)
	case node16:
		return t.removeChild16(n.(*Node16[T]), c)
	case node48:
		return t.removeChild48(n.(*Node48[T]), c)
	case node256:
		return t.removeChild256(n.(*Node256[T]), c)
	default:
		panic("invalid node type")
	}
	return n
}

func (t *RadixTree[T]) removeChild4(n *Node4[T], c byte) Node[T] {
	pos := sort.Search(int(n.numChildren), func(i int) bool {
		return n.keys[i] >= c
	})

	copy(n.keys[pos:], n.keys[pos+1:])
	copy(n.children[pos:], n.children[pos+1:])
	n.numChildren--

	// Remove nodes with only a single child
	if n.numChildren == 1 {
		if n.children[0] == nil {
			return n
		}
		// Is not leaf
		if !n.children[0].isLeaf() {
			// Concatenate the prefixes
			prefix := int(n.getPartialLen())
			if prefix < maxPrefixLen {
				n.partial[prefix] = n.keys[0]
				prefix++
			}
			if prefix < maxPrefixLen {
				subPrefix := min(int(n.children[0].getPartialLen()), maxPrefixLen-prefix)
				copy(n.getPartial()[prefix:], n.children[0].getPartial()[:subPrefix])
				prefix += subPrefix
			}

			// Store the prefix in the child
			copy(n.children[0].getPartial(), n.partial[:min(prefix, maxPrefixLen)])
			n.children[0].setPartialLen(n.children[0].getPartialLen() + n.getPartialLen() + 1)
		}
		return n.children[0]
	}
	return n
}

func (t *RadixTree[T]) removeChild16(n *Node16[T], c byte) Node[T] {
	pos := sort.Search(int(n.numChildren), func(i int) bool {
		return n.keys[i] >= c
	})

	copy(n.keys[pos:], n.keys[pos+1:])
	copy(n.children[pos:], n.children[pos+1:])
	n.numChildren--

	if n.numChildren == 3 {
		newNode := t.allocNode(node4)
		n4 := newNode.(*Node4[T])
		t.copyHeader(newNode, n)
		copy(n4.keys[:], n.keys[:4])
		copy(n4.children[:], n.children[:4])
		return newNode
	}
	return n
}

func (t *RadixTree[T]) removeChild48(n *Node48[T], c uint8) Node[T] {
	pos := n.keys[c]
	n.keys[c] = 0
	n.children[pos-1] = nil
	n.numChildren--

	if n.numChildren == 12 {
		newNode := t.allocNode(node16)
		t.copyHeader(newNode, n)
		child := 0
		for i := 0; i < 256; i++ {
			pos = n.keys[i]
			if pos != 0 {
				newNode.setKeyAtIdx(child, byte(i))
				newNode.setChild(child, n.children[pos-1])
				child++
			}
		}
		return newNode
	}
	return n
}

func (t *RadixTree[T]) removeChild256(n *Node256[T], c uint8) Node[T] {
	n.children[c] = nil
	n.numChildren--

	// Resize to a node48 on underflow, not immediately to prevent
	// trashing if we sit on the 48/49 boundary
	if n.numChildren == 37 {
		newNode := t.allocNode(node48)
		t.copyHeader(newNode, n)

		pos := 0
		for i := 0; i < 256; i++ {
			if n.children[i] != nil {
				newNode.setChild(pos, n.children[i])
				newNode.setKeyAtIdx(i, byte(pos+1))
				pos++
			}
		}
		return newNode
	}
	return n
}
