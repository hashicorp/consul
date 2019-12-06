package tree

import "github.com/miekg/dns"

// Walk performs fn on all authoritative values stored in the tree in
// in-order depth first. If a non-nil error is returned the Walk was interrupted
// by an fn returning that error. If fn alters stored values' sort
// relationships, future tree operation behaviors are undefined.
func (t *Tree) Walk(fn func(*Elem, map[uint16][]dns.RR) error) error {
	if t.Root == nil {
		return nil
	}
	return t.Root.walk(fn)
}

func (n *Node) walk(fn func(*Elem, map[uint16][]dns.RR) error) error {
	if n.Left != nil {
		if err := n.Left.walk(fn); err != nil {
			return err
		}
	}

	if err := fn(n.Elem, n.Elem.m); err != nil {
		return err
	}

	if n.Right != nil {
		if err := n.Right.walk(fn); err != nil {
			return err
		}
	}
	return nil
}
