package tree

// All traverses tree and returns all elements
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

// Do performs fn on all values stored in the tree. A boolean is returned indicating whether the
// Do traversal was interrupted by an Operation returning true. If fn alters stored values' sort
// relationships, future tree operation behaviors are undefined.
func (t *Tree) Do(fn func(e *Elem) bool) bool {
	if t.Root == nil {
		return false
	}
	return t.Root.do(fn)
}

func (n *Node) do(fn func(e *Elem) bool) (done bool) {
	if n.Left != nil {
		done = n.Left.do(fn)
		if done {
			return
		}
	}
	done = fn(n.Elem)
	if done {
		return
	}
	if n.Right != nil {
		done = n.Right.do(fn)
	}
	return
}
