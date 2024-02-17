// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

// Iterator is used to iterate over a set of nodes from the root
// down to a specified path. This will iterate over the same values that
// the Node.WalkPath method will.
type Iterator[T any] struct {
	path  []byte
	root  *Node[T]
	stack []Node[T]
	depth int
}

func (i *Iterator[T]) Next() ([]byte, T, bool) {
	var zero T

	if len(i.stack) == 0 {
		return nil, zero, false
	}

	// Iterate through the stack until it's empty
	for len(i.stack) > 0 {
		node := i.stack[0]
		i.stack = i.stack[1:]
		currentNode := node.(Node[T])

		switch currentNode.getArtNodeType() {
		case LEAF:
			leafCh := currentNode.(*NodeLeaf[T])
			if !leafCh.matchPrefix(i.path) {
				continue
			}
			return getKey(leafCh.key), leafCh.value, true
		case NODE4:
			node4 := currentNode.(*Node4[T])
			for itr := int(node4.getNumChildren()) - 1; itr >= 0; itr-- {
				child := (*node4.children[itr]).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case NODE16:
			node16 := currentNode.(*Node16[T])
			for itr := int(node16.getNumChildren()) - 1; itr >= 0; itr-- {
				child := (*node16.children[itr]).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case NODE48:
			node48 := currentNode.(*Node48[T])
			for itr := int(node48.getNumChildren()) - 1; itr >= 0; itr-- {
				child := (*node48.children[itr]).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case NODE256:
			node256 := currentNode.(*Node256[T])
			for itr := int(node256.getNumChildren()) - 1; itr >= 0; itr-- {
				child := (*node256.children[itr]).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		}
	}
	return nil, zero, false
}

func (i *Iterator[T]) SeekPrefix(prefixKey []byte) {
	// Start from the root node
	prefix := getTreeKey(prefixKey)

	i.path = prefix

	i.stack = nil
	node := *i.root
	depth := 0

	for node != nil {
		// Check if the node matches the prefix
		i.stack = []Node[T]{node}
		i.root = &node

		if node.isLeaf() {
			return
		}

		// Determine the child index to proceed based on the next byte of the prefix
		if node.getPartialLen() > 0 {
			// If the node has a prefix, compare it with the prefix
			mismatchIdx := prefixMismatch[T](node, prefix, len(prefix), depth)
			if mismatchIdx < int(node.getPartialLen()) {
				// If there's a mismatch, set the node to nil to break the loop
				node = nil
				break
			}
			depth += int(node.getPartialLen())
		}

		// Get the next child node based on the prefix
		child := findChild[T](node, prefix[depth])
		if child == nil {
			// If the child node doesn't exist, break the loop
			node = nil
			break
		}

		if depth == len(prefix)-1 {
			// If the prefix is exhausted, break the loop
			break
		}

		// Move to the next level in the tree
		node = **child
		depth++
	}

}
