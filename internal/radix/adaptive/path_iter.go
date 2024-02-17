// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

// PathIterator is used to iterate over a set of nodes from the root
// down to a specified path. This will iterate over the same values that
// the Node.WalkPath method will.
type PathIterator[T any] struct {
	path    []byte
	depth   int
	parent  *Node[T]
	stack   []Node[T]
	current *Node[T]
}

func (i *PathIterator[T]) Next() ([]byte, T, bool) {
	var zero T

	if i.current == nil && len(i.stack) == 0 {
		i.stack = append(i.stack, *i.parent)
		i.current = i.parent
	}

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
			if leafCh.prefixContainsMatch(i.path) {
				return getKey(leafCh.key), leafCh.value, true
			}
			continue
		case NODE4:
			node4 := currentNode.(*Node4[T])
			for itr := 3; itr >= 0; itr-- {
				nodeCh := node4.children[itr]
				if nodeCh == nil {
					continue
				}
				child := (*node4.children[itr]).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case NODE16:
			node16 := currentNode.(*Node16[T])
			for itr := 15; itr >= 0; itr-- {
				nodeCh := node16.children[itr]
				if nodeCh == nil {
					continue
				}
				child := (*nodeCh).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case NODE48:
			node48 := currentNode.(*Node48[T])
			for itr := 47; itr >= 0; itr-- {
				nodeCh := node48.children[itr]
				if nodeCh == nil {
					continue
				}
				child := (*nodeCh).(Node[T])
				newStack := make([]Node[T], len(i.stack)+1)
				copy(newStack[1:], i.stack)
				newStack[0] = child
				i.stack = newStack
			}
		case NODE256:
			node256 := currentNode.(*Node256[T])
			for itr := 255; itr >= 0; itr-- {
				nodeCh := node256.children[itr]
				if nodeCh == nil {
					continue
				}
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
