package tree

// All traverses tree and returns all elements.
func (t *Tree) All() []*Elem {
	if t.Root == nil {
		return nil
	}
	found := t.Root.all(nil)
	return found
}

func (n *Node) all(found []*Elem) []*Elem {
	if n.Left != nil {
		found = n.Left.all(found)
	}
	found = append(found, n.Elem)
	if n.Right != nil {
		found = n.Right.all(found)
	}
	return found
}
