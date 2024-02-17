// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

import (
	"bytes"
	"math/bits"
)

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
			if prefixLen != min(MaxPrefixLen, int(n.getPartialLen())) {
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

func checkPrefix[T any](n Node[T], key []byte, keyLen, depth int) int {
	maxCmp := min(min(int(n.getPartialLen()), MaxPrefixLen), keyLen-depth)
	var idx int
	for idx = 0; idx < maxCmp; idx++ {
		if n.getPartial()[idx] != key[depth+idx] {
			return idx
		}
	}
	return idx
}

func leafMatches[T any](n *NodeLeaf[T], key []byte, keyLen int) int {
	// Fail if the key lengths are different
	if int(n.keyLen) != keyLen {
		return 1
	}
	// Compare the keys
	return bytes.Compare(n.key, key)
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
	// This means root is nil
	if node.isLeaf() {
		nodeLeaf := node.(*NodeLeaf[T])
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
		if bytes.Equal(nodeLeaf.key, key[:keyLen]) {
			*old = 1
			oldVal := nodeLeaf.value
			nodeLeaf.value = value
			return oldVal
		}

		// New value, we must split the leaf into a node4
		newLeaf2 := t.makeLeaf(key, value).(*NodeLeaf[T])

		// Determine longest prefix
		longestPrefix := longestCommonPrefix[T](nodeLeaf, newLeaf2, depth)
		newNode := t.allocNode(NODE4)
		newNode4 := newNode.(*Node4[T])
		newNode4.partialLen = uint32(longestPrefix)
		copy(newNode4.partial[:], key[depth:depth+min(MaxPrefixLen, longestPrefix)])

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
			goto RECURSE_SEARCH
		}

		// Create a new node
		newNode := t.allocNode(NODE4)
		*ref = &newNode
		newNode4 := newNode.(*Node4[T])
		newNode4.partialLen = uint32(prefixDiff)
		copy(newNode4.partial[:], node.getPartial()[:min(MaxPrefixLen, prefixDiff)])

		// Adjust the prefix of the old node
		if node.getPartialLen() <= MaxPrefixLen {
			t.addChild4(newNode4, ref, node.getPartial()[prefixDiff], node)
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			length := min(MaxPrefixLen, int(node.getPartialLen()))
			copy(node.getPartial()[:], node.getPartial()[prefixDiff+1:+prefixDiff+1+length])
		} else {
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			l := minimum[T](&node)
			if l == nil {
				return zero
			}
			t.addChild4(newNode4, ref, l.key[depth+prefixDiff], node)
			length := min(MaxPrefixLen, int(node.getPartialLen()))
			copy(node.getPartial()[:], l.key[depth+prefixDiff+1:depth+prefixDiff+1+length])
		}
		// Insert the new leaf
		newLeaf := t.makeLeaf(key, value)
		t.addChild4(newNode4, ref, key[depth+prefixDiff], newLeaf)
		return zero
	}

RECURSE_SEARCH:
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

func (t *RadixTree[T]) makeLeaf(key []byte, value T) Node[T] {
	// Allocate memory for the leaf node
	l := t.allocNode(LEAF).(*NodeLeaf[T])
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

func (t *RadixTree[T]) allocNode(nodeType uint8) Node[T] {
	var n Node[T]
	switch nodeType {
	case LEAF:
		n = &NodeLeaf[T]{
			mutex: t.mutex,
		}
	case NODE4:
		n = &Node4[T]{
			mutex: t.mutex,
		}
	case NODE16:
		n = &Node16[T]{
			mutex: t.mutex,
		}
	case NODE48:
		n = &Node48[T]{
			mutex: t.mutex,
		}
	case NODE256:
		n = &Node256[T]{
			mutex: t.mutex,
		}
	default:
		panic("Unknown node type")
	}
	n.setArtNodeType(nodeType)
	n.setPartial(make([]byte, MaxPrefixLen))
	n.setPartialLen(MaxPrefixLen)
	return n
}

// longestCommonPrefix finds the length of the longest common prefix between two leaf nodes.
func longestCommonPrefix[T any](l1, l2 *NodeLeaf[T], depth int) int {
	maxCmp := int(l2.keyLen) - depth
	if int(l1.keyLen) < int(l2.keyLen) {
		maxCmp = int(l1.keyLen) - depth
	}
	var idx int
	for idx = 0; idx < maxCmp; idx++ {
		if l1.key[depth+idx] != l2.key[depth+idx] {
			return idx
		}
	}
	return idx
}

// addChild adds a child node to the parent node.
func (t *RadixTree[T]) addChild(n Node[T], ref **Node[T], c byte, child Node[T]) {
	switch n.getArtNodeType() {
	case NODE4:
		t.addChild4(n.(*Node4[T]), ref, c, child)
	case NODE16:
		t.addChild16(n.(*Node16[T]), ref, c, child)
	case NODE48:
		t.addChild48(n.(*Node48[T]), ref, c, child)
	case NODE256:
		t.addChild256(n.(*Node256[T]), ref, c, child)
	default:
		panic("Unknown node type")
	}
}

// addChild4 adds a child node to a node4.
func (t *RadixTree[T]) addChild4(n *Node4[T], ref **Node[T], c byte, child Node[T]) {
	if n.numChildren < 4 {
		idx := 0
		for idx = 0; idx < int(n.numChildren); idx++ {
			if c < n.keys[idx] {
				break
			}
		}

		// Shift to make room
		length := int(n.numChildren) - idx
		copy(n.keys[idx+1:], n.keys[idx:idx+length])
		copy(n.children[idx+1:], n.children[idx:idx+length])

		// Insert element
		n.keys[idx] = c
		n.children[idx] = &child
		n.numChildren++

	} else {
		newNode := t.allocNode(NODE16)
		*ref = &newNode
		node16 := newNode.(*Node16[T])
		// Copy the child pointers and the key map
		copy(node16.children[:], n.children[:n.numChildren])
		copy(node16.keys[:], n.keys[:n.numChildren])
		t.copyHeader(newNode, n)
		t.addChild16(node16, ref, c, child)
	}
}

// addChild16 adds a child node to a node16.
func (t *RadixTree[T]) addChild16(n *Node16[T], ref **Node[T], c byte, child Node[T]) {
	if n.numChildren < 16 {
		var mask uint32 = (1 << n.numChildren) - 1
		var bitfield uint32

		// Compare the key to all 16 stored keys
		for i := 0; i < 16; i++ {
			if c < n.keys[i] {
				bitfield |= 1 << i
			}
		}

		// Use a mask to ignore children that don't exist
		bitfield &= mask

		// Check if less than any
		var idx int
		if bitfield != 0 {
			idx = bits.TrailingZeros32(bitfield)
			length := int(n.numChildren) - idx
			copy(n.keys[idx+1:], n.keys[idx:idx+length])
			copy(n.children[idx+1:], n.children[idx:idx+length])
		} else {
			idx = int(n.numChildren)
		}

		// Set the child
		n.keys[idx] = c
		n.children[idx] = &child
		n.numChildren++

	} else {
		newNode := t.allocNode(NODE48)
		*ref = &newNode

		node48 := newNode.(*Node48[T])
		// Copy the child pointers and populate the key map
		copy(node48.children[:], n.children[:n.numChildren])
		for i := 0; i < int(n.numChildren); i++ {
			node48.keys[n.keys[i]] = byte(i + 1)
		}

		t.copyHeader(newNode, n)
		t.addChild48(node48, ref, c, child)
	}
}

// addChild48 adds a child node to a node48.
func (t *RadixTree[T]) addChild48(n *Node48[T], ref **Node[T], c byte, child Node[T]) {
	if n.numChildren < 48 {
		pos := 0
		for n.children[pos] != nil {
			pos++
		}
		n.children[pos] = &child
		n.keys[c] = byte(pos + 1)
		n.numChildren++
	} else {
		newNode := t.allocNode(NODE256)
		*ref = &newNode
		node256 := newNode.(*Node256[T])
		for i := 0; i < 256; i++ {
			if n.keys[i] != 0 {
				node256.children[i] = n.children[int(n.keys[i])-1]
			}
		}
		t.copyHeader(newNode, n)
		t.addChild256(node256, ref, c, child)
	}
}

// copyHeader copies header information from src to dest node.
func (t *RadixTree[T]) copyHeader(dest, src Node[T]) {
	dest.setNumChildren(src.getNumChildren())
	dest.setPartialLen(src.getPartialLen())
	length := min(MaxPrefixLen, int(src.getPartialLen()))
	partialToCopy := src.getPartial()[:length]
	copy(dest.getPartial()[:length], partialToCopy)
}

// addChild256 adds a child node to a node256.
func (t *RadixTree[T]) addChild256(n *Node256[T], _ **Node[T], c byte, child Node[T]) {
	n.numChildren++
	n.children[c] = &child
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// prefixMismatch calculates the index at which the prefixes mismatch.
func prefixMismatch[T any](n Node[T], key []byte, keyLen, depth int) int {
	maxCmp := min(min(MaxPrefixLen, int(n.getPartialLen())), keyLen-depth)
	var idx int
	for idx = 0; idx < maxCmp; idx++ {
		if n.getPartial()[idx] != key[depth+idx] {
			return idx
		}
	}

	// If the prefix is short we can avoid finding a leaf
	if n.getPartialLen() > MaxPrefixLen {
		// Prefix is longer than what we've checked, find a leaf
		l := minimum(&n)
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
func minimum[T any](n *Node[T]) *NodeLeaf[T] {
	// Handle base cases
	if n == nil {
		return nil
	}
	node := *n
	if isLeaf[T](node) {
		return node.(*NodeLeaf[T])
	}

	var idx int
	switch node.getArtNodeType() {
	case NODE4:
		return minimum[T](node.(*Node4[T]).children[0])
	case NODE16:
		return minimum[T](node.(*Node16[T]).children[0])
	case NODE48:
		idx = 0
		node := node.(*Node48[T])
		for idx < 48 && node.children[idx] == nil {
			idx++
		}
		if idx < 256 {
			return minimum[T](node.children[idx])
		}
	case NODE256:
		node := node.(*Node256[T])
		idx = 0
		for idx < 256 && node.children[idx] == nil {
			idx++
		}
		if idx < 256 {
			return minimum[T](node.children[idx])
		}
	default:
		panic("Unknown node type")
	}
	return nil
}

// maximum finds the maximum leaf under a node.
func maximum[T any](n *Node[T]) *NodeLeaf[T] {
	// Handle base cases
	if n == nil {
		return nil
	}

	node := *n

	if isLeaf[T](node) {
		return node.(*NodeLeaf[T])
	}
	var idx int
	switch node.getArtNodeType() {
	case NODE4:
		return maximum[T](node.(*Node4[T]).children[node.getNumChildren()-1])
	case NODE16:
		return maximum[T](node.(*Node16[T]).children[node.getNumChildren()-1])
	case NODE48:
		node := node.(*Node48[T])
		idx = 255
		for idx >= 0 && node.children[idx] == nil {
			idx--
		}
		if idx >= 0 {
			return maximum[T](node.children[idx])
		}
	case NODE256:
		idx = 255
		node := node.(*Node256[T])
		for idx >= 0 && node.children[idx] == nil {
			idx--
		}
		if idx >= 0 {
			return maximum[T](node.children[idx])
		}
	default:
		panic("Unknown node type")
	}
	return nil
}

// IS_LEAF checks whether the least significant bit of the pointer x is set.
func isLeaf[T any](node Node[T]) bool {
	return node.isLeaf()
}

// findChild finds the child node pointer based on the given character in the ART tree node.
func (t *RadixTree[T]) findChild(n Node[T], c byte) **Node[T] {
	return findChild(n, c)
}
func findChild[T any](n Node[T], c byte) **Node[T] {
	switch n.getArtNodeType() {
	case NODE4:
		node := n.(*Node4[T])
		for i := 0; i < int(n.getNumChildren()); i++ {
			if node.keys[i] == c {
				return &node.children[i]
			}
		}
	case NODE16:
		node := n.(*Node16[T])

		// Compare the key to all 16 stored keys
		var bitfield uint16
		for i := 0; i < int(n.getNumChildren()); i++ {
			if node.keys[i] == c {
				bitfield |= 1 << uint(i)
			}
		}

		// Use a mask to ignore children that don't exist
		mask := (1 << n.getNumChildren()) - 1
		bitfield &= uint16(mask)

		// If we have a match (any bit set), return the pointer match
		if bitfield != 0 {
			return &node.children[bits.TrailingZeros16(bitfield)]
		}
	case NODE48:
		node := n.(*Node48[T])
		i := node.keys[c]
		if i != 0 {
			return &node.children[i-1]
		}
	case NODE256:
		node := n.(*Node256[T])
		if node.children[c] != nil {
			return &node.children[c]
		}
	case LEAF:
		// no-op
		return nil
	default:
		panic("Unknown node type")
	}
	return nil
}

func getTreeKey(key []byte) []byte {
	keyLen := len(key) + 1
	newKey := make([]byte, keyLen)
	copy(newKey, key)
	newKey[keyLen-1] = '$'
	return newKey
}

func getKey(key []byte) []byte {
	keyLen := len(key)
	if keyLen == 0 {
		return nil
	}
	newKey := make([]byte, keyLen-1)
	copy(newKey, key)
	return newKey
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
		if prefixLen != min(MaxPrefixLen, int(node.getPartialLen())) {
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

func (t *RadixTree[T]) removeChild(n Node[T], ref **Node[T], c byte, l **Node[T]) {
	switch n.getArtNodeType() {
	case NODE4:
		t.removeChild4(n.(*Node4[T]), ref, l)
	case NODE16:
		t.removeChild16(n.(*Node16[T]), ref, l)
	case NODE48:
		t.removeChild48(n.(*Node48[T]), ref, c)
	case NODE256:
		t.removeChild256(n.(*Node256[T]), ref, c)
	default:
		panic("invalid node type")
	}
}

func (t *RadixTree[T]) removeChild4(n *Node4[T], ref **Node[T], l **Node[T]) {
	pos := -1
	for i, node := range n.children {
		if node == *l {
			pos = i
			break
		}
	}
	if pos == -1 {
		return // Child node not found
	}

	node := *n
	copy(n.keys[pos:], n.keys[pos+1:])
	copy(n.children[pos:], n.children[pos+1:])
	node.numChildren--

	// Remove nodes with only a single child
	if node.numChildren == 1 {
		if n.children[0] == nil {
			return
		}
		child := *n.children[0]
		// Is not leaf
		if !child.isLeaf() {
			// Concatenate the prefixes
			prefix := int(node.getPartialLen())
			if prefix < MaxPrefixLen {
				n.partial[prefix] = n.keys[0]
				prefix++
			}
			if prefix < MaxPrefixLen {
				subPrefix := min(int(child.getPartialLen()), MaxPrefixLen-prefix)
				copy(node.getPartial()[prefix:], child.getPartial()[:subPrefix])
				prefix += subPrefix
			}

			// Store the prefix in the child
			copy(child.getPartial(), node.partial[:min(prefix, MaxPrefixLen)])
			child.setPartialLen(child.getPartialLen() + node.getPartialLen() + 1)
		}
		*ref = &child
	}
}

func (t *RadixTree[T]) removeChild16(n *Node16[T], ref **Node[T], l **Node[T]) {
	pos := -1
	for i, node := range n.children {
		if node == *l {
			pos = i
			break
		}
	}
	if pos == -1 {
		return // Child node not found
	}

	node := *n
	copy(n.keys[pos:], n.keys[pos+1:])
	copy(n.children[pos:], n.children[pos+1:])
	node.numChildren--

	if node.numChildren == 3 {
		newNode := t.allocNode(NODE4)
		*ref = &newNode
		node4 := newNode.(*Node4[T])
		t.copyHeader(newNode, n)
		copy(node4.keys[:], node.keys[:4])
		copy(node4.children[:], node.children[:4])
	}
}

func (t *RadixTree[T]) removeChild48(n *Node48[T], ref **Node[T], c uint8) {
	pos := n.keys[c]
	n.keys[c] = 0
	n.children[pos-1] = nil
	n.numChildren--

	if n.numChildren == 12 {
		newNode := t.allocNode(NODE16)
		*ref = &newNode
		node16 := newNode.(*Node16[T])
		t.copyHeader(newNode, n)

		child := 0
		for i := 0; i < 256; i++ {
			pos = n.keys[i]
			if pos != 0 {
				node16.keys[child] = byte(i)
				node16.children[child] = n.children[pos-1]
				child++
			}
		}
	}
}

func (t *RadixTree[T]) removeChild256(n *Node256[T], ref **Node[T], c uint8) {
	n.children[c] = nil
	n.numChildren--

	// Resize to a node48 on underflow, not immediately to prevent
	// trashing if we sit on the 48/49 boundary
	if n.numChildren == 37 {
		newNode := t.allocNode(NODE48)
		*ref = &newNode
		node48 := newNode.(*Node48[T])
		t.copyHeader(newNode, n)

		pos := 0
		for i := 0; i < 256; i++ {
			if n.children[i] != nil {
				node48.children[pos] = n.children[i]
				node48.keys[i] = byte(pos + 1)
				pos++
			}
		}
	}
}
