package tree

import (
	"github.com/miekg/dns"
)

// AuthWalk performs fn on all authoritative values stored in the tree in
// pre-order depth first. If a non-nil error is returned the AuthWalk was interrupted
// by an fn returning that error. If fn alters stored values' sort
// relationships, future tree operation behaviors are undefined.
//
// The fn function will be called with 3 arguments, the current element, a map containing all
// the RRs for this element and a boolean if this name is considered authoritative.
func (t *Tree) AuthWalk(fn func(*Elem, map[uint16][]dns.RR, bool) error) error {
	if t.Root == nil {
		return nil
	}
	return t.Root.authwalk(make(map[string]struct{}), fn)
}

func (n *Node) authwalk(ns map[string]struct{}, fn func(*Elem, map[uint16][]dns.RR, bool) error) error {
	if n.Left != nil {
		if err := n.Left.authwalk(ns, fn); err != nil {
			return err
		}
	}

	// Check if the current name is a subdomain of *any* of the delegated names we've seen, if so, skip this name.
	// The ordering of the tree and how we walk if guarantees we see parents first.
	if n.Elem.Type(dns.TypeNS) != nil {
		ns[n.Elem.Name()] = struct{}{}
	}

	auth := true
	i := 0
	for {
		j, end := dns.NextLabel(n.Elem.Name(), i)
		if end {
			break
		}
		if _, ok := ns[n.Elem.Name()[j:]]; ok {
			auth = false
			break
		}
		i++
	}

	if err := fn(n.Elem, n.Elem.m, auth); err != nil {
		return err
	}

	if n.Right != nil {
		if err := n.Right.authwalk(ns, fn); err != nil {
			return err
		}
	}
	return nil
}
