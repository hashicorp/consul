// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

// PathIterator is used to iterate over a set of nodes from the root
// down to a specified path. This will iterate over the same values that
// the Node.WalkPath method will.
type PathIterator[T any] struct {
	path      []byte
	depth     int
	currentCh uint8
	parent    *Node[T]
}

// Next returns the next node in order
func (i *PathIterator[T]) recursiveForEachChildren(child *Node[T]) ([]byte, T, bool) {
	var zero T
	childNode := *child
	if childNode != nil && childNode.isLeaf() {
		leaf := childNode.(*NodeLeaf[T])
		if leaf.prefixMatch(i.path) {
			return getKey(leaf.key), leaf.value, true
		}
	} else {
		i.parent = child
		i.currentCh = 0
		key, value, found := i.recursiveForEach()
		if found {
			return key, value, true
		}
	}
	return nil, zero, false
}

func (i *PathIterator[T]) Next() ([]byte, T, bool) {
	var zero T
	key, value, found := i.recursiveForEach()
	if found {
		i.Iterate()
		return key, value, true
	}
	return nil, zero, false
}

func (i *PathIterator[T]) Iterate() {
	depth := i.depth
	if *i.parent == nil {
		return
	}
	parent := *i.parent
	if parent.getNumChildren() > i.currentCh {
		i.currentCh++
		return
	}
	if parent.getPartialLen() > 0 {
		prefixLen := checkPrefix[T](parent, i.path, len(i.path), i.depth)
		if prefixLen != min(MaxPrefixLen, int(parent.getPartialLen())) {
			return
		}
		depth += int(parent.getPartialLen())
	}
	next := findChild(parent, i.path[depth])
	if next == nil {
		i.parent = nil
		i.currentCh = 0
		return
	}
	i.parent = *next
	i.currentCh = 0
	depth++
	i.depth = depth
}

func (i *PathIterator[T]) recursiveForEach() ([]byte, T, bool) {
	var zero T

	if i.parent == nil || *i.parent == nil {
		return nil, zero, false
	}

	parent := *i.parent

	if parent.getNumChildren() == i.currentCh {
		i.Iterate()
		return i.Next()
	}

	switch parent.getArtNodeType() {
	case NODE4:
		node4 := parent.(*Node4[T])
		for itr := i.currentCh; itr < node4.numChildren; itr++ {
			if node4.children[itr] != nil {
				key, value, found := i.recursiveForEachChildren(node4.children[itr])
				if found {
					return key, value, true
				}
			}
		}

	case NODE16:
		node16 := parent.(*Node16[T])
		for itr := i.currentCh; itr < node16.numChildren; itr++ {
			if node16.children[itr] != nil {
				i.currentCh = itr
				key, value, found := i.recursiveForEachChildren(node16.children[itr])
				if found {
					return key, value, true
				}
			}
		}

	case NODE48:
		node48 := parent.(*Node48[T])
		for itr := i.currentCh; itr < node48.numChildren; itr++ {
			if node48.children[itr] != nil {
				i.currentCh = itr
				key, value, found := i.recursiveForEachChildren(node48.children[itr])
				if found {
					return key, value, true
				}
			}
		}

	case NODE256:
		node256 := parent.(*Node256[T])
		for itr := i.currentCh; itr < node256.numChildren; itr++ {
			if node256.children[itr] != nil {
				i.currentCh = itr
				key, value, found := i.recursiveForEachChildren(node256.children[itr])
				if found {
					return key, value, true
				}
			}
		}
	}
	return nil, zero, false
}
