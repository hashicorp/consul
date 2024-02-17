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
			vis := [4]bool{}
			for {
				indx := getMinKeyIndexNode4(node4.keys, vis)
				if indx == -1 || indx >= 16 {
					break
				}
				vis[indx] = true
				nodeCh := node4.children[indx]
				if nodeCh == nil {
					continue
				}
				child := (*node4.children[indx]).(Node[T])
				i.stack = append(i.stack, child)
			}
		case NODE16:
			node16 := currentNode.(*Node16[T])
			vis := [16]bool{}
			for {
				indx := getMinKeyIndexNode16(node16.keys, vis)
				if indx == -1 || indx >= 16 {
					break
				}
				vis[indx] = true
				nodeCh := node16.children[indx]
				if nodeCh == nil {
					continue
				}
				child := (*node16.children[indx]).(Node[T])
				i.stack = append(i.stack, child)
			}
		case NODE48:
			node48 := currentNode.(*Node48[T])
			vis := [256]bool{}
			for {
				indx := getMinKeyIndexNode48(node48.keys, vis)
				if indx == -1 {
					break
				}
				vis[indx] = true
				nodeCh := node48.children[node48.keys[indx]]
				if nodeCh == nil {
					continue
				}
				child := (*node48.children[node48.keys[indx]]).(Node[T])
				i.stack = append(i.stack, child)
			}
		case NODE256:
			node256 := currentNode.(*Node256[T])
			for itr := int(node256.getNumChildren()) - 1; itr >= 0; itr-- {
				nodeCh := node256.children[itr]
				if nodeCh == nil {
					continue
				}
				child := (*node256.children[itr]).(Node[T])
				i.stack = append(i.stack, child)
			}
		}
	}
	return nil, zero, false
}

func getMinKeyIndexNode4(keys [4]byte, vis [4]bool) int {
	minV := byte(255)
	indx := -1
	for i := 0; i < 4; i++ {
		if keys[i] != 0 && minV > keys[i] && !vis[i] {
			minV = keys[i]
			indx = i
		}
	}
	return indx
}

func getMinKeyIndexNode16(keys [16]byte, vis [16]bool) int {
	minV := byte(255)
	indx := -1
	for i := 0; i < 16; i++ {
		if keys[i] != 0 && minV > keys[i] && !vis[i] {
			minV = keys[i]
			indx = i
		}
	}
	return indx
}

func getMinKeyIndexNode48(keys [256]byte, vis [256]bool) int {
	return getMinKeyIndexNode256(keys, vis)
}

func getMinKeyIndexNode256(keys [256]byte, vis [256]bool) int {
	minV := byte(255)
	indx := -1
	for i := 0; i < 256; i++ {
		if keys[i] != 0 && minV > keys[i] && !vis[i] {
			minV = keys[i]
			indx = i
		}
	}
	return indx
}
