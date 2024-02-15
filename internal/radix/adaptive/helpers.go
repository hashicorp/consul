package adaptive

import (
	"bytes"
	"math/bits"
)

func iterativeSearch[T any](t *RadixTree[T], key []byte) T {
	var nilT T
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
				return leaf.value
			}
			return nilT
		}

		// Bail if the prefix does not match
		if n.getPartialLen() > 0 {
			prefixLen := checkPrefix[T](n, key, keyLen, depth)
			if prefixLen != min(MaxPrefixLen, int(n.getPartialLen())) {
				return nilT
			}
			depth += int(n.getPartialLen())
		}

		// Recursively search
		child = findChild[T](n, key[depth])
		if child != nil {
			n = **child
		} else {
			n = nil
		}
		depth++
	}
	return nilT
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

func recursiveInsert[T any](n *Node[T], ref **Node[T], key []byte, value T, depth int, old *int) T {
	var nilT T
	keyLen := len(key)
	// If we are at a nil node, inject a leaf
	if n == nil {
		leafNode := makeLeaf[T](key, value)
		*ref = &leafNode
		return nilT
	}

	// If we are at a leaf, we need to replace it with a node
	node := *n
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
		newLeaf2 := makeLeaf[T](key, value).(*NodeLeaf[T])

		// Determine longest prefix
		longestPrefix := longestCommonPrefix[T](nodeLeaf, newLeaf2, depth)
		newNode := allocNode[T](NODE4)
		newNode4 := newNode.(*Node4[T])
		newNode4.partialLen = uint32(longestPrefix)
		copy(newNode4.partial[:], key[depth:depth+min(MaxPrefixLen, longestPrefix)])

		// Add the leafs to the new node4
		addChild4[T](newNode4, ref, nodeLeaf.key[depth+longestPrefix], nodeLeaf)
		addChild4[T](newNode4, ref, newLeaf2.key[depth+longestPrefix], newLeaf2)
		*ref = &newNode
		return nilT
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
		newNode := allocNode[T](NODE4)
		*ref = &newNode
		newNode4 := newNode.(*Node4[T])
		newNode4.partialLen = uint32(prefixDiff)
		copy(newNode4.partial[:], node.getPartial()[:min(MaxPrefixLen, prefixDiff)])

		// Adjust the prefix of the old node
		if node.getPartialLen() <= MaxPrefixLen {
			addChild4[T](newNode4, ref, node.getPartial()[prefixDiff], node)
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			copy(node.getPartial()[:], node.getPartial()[prefixDiff+1:min(MaxPrefixLen, int(node.getPartialLen())+prefixDiff+1)])
		} else {
			node.setPartialLen(node.getPartialLen() - uint32(prefixDiff+1))
			l := minimum[T](node)
			addChild4[T](newNode4, ref, l.key[depth+prefixDiff], node)
			copy(node.getPartial()[:], l.key[depth+prefixDiff+1:depth+prefixDiff+1+min(MaxPrefixLen, int(node.getPartialLen()))])
		}
		// Insert the new leaf
		newLeaf := makeLeaf[T](key, value)
		addChild4[T](newNode4, ref, key[depth+prefixDiff], newLeaf)
		return nilT
	}

RECURSE_SEARCH:
	// Find a child to recurse to
	child := findChild[T](node, key[depth])
	if child != nil {
		return recursiveInsert[T](*child, child, key, value, depth+1, old)
	}

	// No child, node goes within us
	newLeaf := makeLeaf[T](key, value)
	addChild[T](node, ref, key[depth], newLeaf)
	return nilT
}

func makeLeaf[T any](key []byte, value T) Node[T] {
	// Allocate memory for the leaf node
	l := allocNode[T](LEAF).(*NodeLeaf[T])
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

func allocNode[T any](nodeType uint8) Node[T] {
	var n Node[T]
	switch nodeType {
	case LEAF:
		n = &NodeLeaf[T]{}
	case NODE4:
		n = &Node4[T]{}
	case NODE16:
		n = &Node16[T]{}
	case NODE48:
		n = &Node48[T]{}
	case NODE256:
		n = &Node256[T]{}
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
func addChild[T any](n Node[T], ref **Node[T], c byte, child Node[T]) {
	switch n.getArtNodeType() {
	case NODE4:
		addChild4[T](n.(*Node4[T]), ref, c, child)
	case NODE16:
		addChild16[T](n.(*Node16[T]), ref, c, child)
	case NODE48:
		addChild48[T](n.(*Node48[T]), ref, c, child)
	case NODE256:
		addChild256[T](n.(*Node256[T]), ref, c, child)
	default:
		panic("Unknown node type")
	}
}

// addChild4 adds a child node to a node4.
func addChild4[T any](n *Node4[T], ref **Node[T], c byte, child Node[T]) {
	if n.numChildren < 4 {
		idx := 0
		for idx = 0; idx < int(n.numChildren); idx++ {
			if c < n.keys[idx] {
				break
			}
		}

		// Shift to make room
		copy(n.keys[idx+1:], n.keys[idx:n.numChildren])
		copy(n.children[idx+1:], n.children[idx:n.numChildren])

		// Insert element
		n.keys[idx] = c
		n.children[idx] = &child
		n.numChildren++

	} else {
		newNode := allocNode[T](NODE16)
		*ref = &newNode
		node16 := newNode.(*Node16[T])
		// Copy the child pointers and the key map
		copy(node16.children[:], n.children[:n.numChildren])
		copy(node16.keys[:], n.keys[:n.numChildren])
		copyHeader[T](newNode, n)
		addChild16[T](node16, ref, c, child)
	}
}

// addChild16 adds a child node to a node16.
func addChild16[T any](n *Node16[T], ref **Node[T], c byte, child Node[T]) {
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
			copy(n.keys[idx+1:], n.keys[idx:])
			copy(n.children[idx+1:], n.children[idx:])
		} else {
			idx = int(n.numChildren)
		}

		// Set the child
		n.keys[idx] = c
		n.children[idx] = &child
		n.numChildren++

	} else {
		newNode := allocNode[T](NODE48)
		*ref = &newNode

		node48 := newNode.(*Node48[T])
		// Copy the child pointers and populate the key map
		copy(node48.children[:], n.children[:n.numChildren])
		for i := 0; i < int(n.numChildren); i++ {
			node48.keys[n.keys[i]] = byte(i + 1)
		}

		copyHeader[T](newNode, n)
		addChild48[T](node48, ref, c, child)
	}
}

// addChild48 adds a child node to a node48.
func addChild48[T any](n *Node48[T], ref **Node[T], c byte, child Node[T]) {
	if n.numChildren < 48 {
		pos := 0
		for n.children[pos] != nil {
			pos++
		}
		n.children[pos] = &child
		n.keys[c] = byte(pos + 1)
		n.numChildren++
	} else {
		newNode := allocNode[T](NODE256)
		*ref = &newNode
		node256 := newNode.(*Node256[T])
		for i := 0; i < 256; i++ {
			if n.keys[i] != 0 {
				node256.children[i] = n.children[int(n.keys[i])-1]
			}
		}
		copyHeader[T](newNode, n)
		addChild256[T](node256, ref, c, child)
	}
}

// copyHeader copies header information from src to dest node.
func copyHeader[T any](dest, src Node[T]) {
	dest.setNumChildren(src.getNumChildren())
	dest.setPartialLen(src.getPartialLen())
	partialToCopy := src.getPartial()[:min(MaxPrefixLen, int(src.getPartialLen()))]
	copy(dest.getPartial()[:min(MaxPrefixLen, int(src.getPartialLen()))], partialToCopy)
}

// addChild256 adds a child node to a node256.
func addChild256[T any](n *Node256[T], ref **Node[T], c byte, child Node[T]) {
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
		l := minimum(n)
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
	if isLeaf[T](n) {
		return n.(*NodeLeaf[T])
	}

	var idx int
	switch n.getArtNodeType() {
	case NODE4:
		return minimum[T](*(n.(*Node4[T])).children[0])
	case NODE16:
		return minimum[T](*(n.(*Node16[T])).children[0])
	case NODE48:
		idx = 0
		node := n.(*Node48[T])
		for idx < 256 && node.children[idx] == nil {
			idx++
		}
		if idx < 256 {
			return minimum[T](*node.children[idx])
		}
	case NODE256:
		node := n.(*Node256[T])
		idx = 0
		for idx < 256 && node.children[idx] == nil {
			idx++
		}
		if idx < 256 {
			return minimum[T](*node.children[idx])
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
	if isLeaf[T](n) {
		return n.(*NodeLeaf[T])
	}

	var idx int
	switch n.getArtNodeType() {
	case NODE4:
		return maximum[T](*n.(*Node4[T]).children[n.getNumChildren()-1])
	case NODE16:
		return maximum[T](*n.(*Node16[T]).children[n.getNumChildren()-1])
	case NODE48:
		node := n.(*Node48[T])
		idx = 255
		for idx >= 0 && *node.children[idx] == nil {
			idx--
		}
		if idx >= 0 {
			return maximum[T](*node.children[idx])
		}
	case NODE256:
		idx = 255
		node := n.(*Node256[T])
		for idx >= 0 && node.children[idx] == nil {
			idx--
		}
		if idx >= 0 {
			return maximum[T](*node.children[idx])
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
	default:
		panic("Unknown node type")
	}
	return nil
}

func getTreeKey(key []byte) []byte {
	keyLen := len(key) + 1
	newKey := make([]byte, keyLen)
	copy(newKey, key)
	return newKey
}
