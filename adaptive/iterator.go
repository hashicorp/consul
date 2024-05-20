// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

// Iterator is used to iterate over a set of nodes from the node
// down to a specified path. This will iterate over the same values that
// the Node.WalkPath method will.
type Iterator[T any] struct {
	path  []byte
	node  Node[T]
	stack []Node[T]
	depth int
	pos   Node[T]
}

// Front returns the current node that has been iterated to.
func (i *Iterator[T]) Front() Node[T] {
	return i.pos
}

func (i *Iterator[T]) Path() string {
	return string(i.path)
}

// Next
// TODO - Update the i.pos to the current node
// TODO - Update the i.path to the current path
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
		case leafType:
			leafCh := currentNode.(*NodeLeaf[T])
			if !leafCh.matchPrefix(i.path) {
				continue
			}
			return getKey(leafCh.key), leafCh.value, true
		case node4:
			n4 := currentNode.(*Node4[T])
			for itr := 3; itr >= 0; itr-- {
				nodeCh := n4.children[itr]
				if nodeCh == nil {
					continue
				}
				child := (n4.children[itr]).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case node16:
			n16 := currentNode.(*Node16[T])
			for itr := 15; itr >= 0; itr-- {
				nodeCh := n16.children[itr]
				if nodeCh == nil {
					continue
				}
				child := (nodeCh).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case node48:
			n48 := currentNode.(*Node48[T])
			for itr := 0; itr < 256; itr++ {
				idx := n48.keys[itr]
				if idx == 0 {
					continue
				}
				nodeCh := n48.children[idx-1]
				if nodeCh == nil {
					continue
				}
				child := (nodeCh).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case node256:
			n256 := currentNode.(*Node256[T])
			for itr := 255; itr >= 0; itr-- {
				nodeCh := n256.children[itr]
				if nodeCh == nil {
					continue
				}
				child := (n256.children[itr]).(Node[T])
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
	// Start from the node node
	node := i.node

	prefix := getTreeKey(prefixKey)

	i.path = prefix

	i.stack = nil
	depth := 0

	for node != nil {
		// Check if the node matches the prefix
		i.stack = []Node[T]{node}
		i.node = node

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
		child, _ := findChild[T](node, prefix[depth])
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
		node = child
		depth++
	}
}
