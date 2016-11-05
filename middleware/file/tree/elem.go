package tree

import "github.com/miekg/dns"

// Elem is an element in the tree.
type Elem struct {
	m    map[uint16][]dns.RR
	name string // owner name
}

// newElem returns a new elem.
func newElem(rr dns.RR) *Elem {
	e := Elem{m: make(map[uint16][]dns.RR)}
	e.m[rr.Header().Rrtype] = []dns.RR{rr}
	return &e
}

// Types returns the RRs with type qtype from e. If qname is given (only the
// first one is used), the RR are copied and the owner is replaced with qname[0].
func (e *Elem) Types(qtype uint16, qname ...string) []dns.RR {
	rrs := e.m[qtype]

	if rrs != nil && len(qname) > 0 {
		copied := make([]dns.RR, len(rrs))
		for i := range rrs {
			copied[i] = dns.Copy(rrs[i])
			copied[i].Header().Name = qname[0]
		}
		return copied
	}
	return rrs
}

// All returns all RRs from e, regardless of type.
func (e *Elem) All() []dns.RR {
	list := []dns.RR{}
	for _, rrs := range e.m {
		list = append(list, rrs...)
	}
	return list
}

// Name returns the name for this node.
func (e *Elem) Name() string {
	if e.name != "" {
		return e.name
	}
	for _, rrs := range e.m {
		e.name = rrs[0].Header().Name
		return e.name
	}
	return ""
}

// Empty returns true is e does not contain any RRs, i.e. is an
// empty-non-terminal.
func (e *Elem) Empty() bool {
	return len(e.m) == 0
}

// Insert inserts rr into e. If rr is equal to existing rrs this is a noop.
func (e *Elem) Insert(rr dns.RR) {
	t := rr.Header().Rrtype
	if e.m == nil {
		e.m = make(map[uint16][]dns.RR)
		e.m[t] = []dns.RR{rr}
		return
	}
	rrs, ok := e.m[t]
	if !ok {
		e.m[t] = []dns.RR{rr}
		return
	}
	for _, er := range rrs {
		if equalRdata(er, rr) {
			return
		}
	}

	rrs = append(rrs, rr)
	e.m[t] = rrs
}

// Delete removes rr from e. When e is empty after the removal the returned bool is true.
func (e *Elem) Delete(rr dns.RR) (empty bool) {
	if e.m == nil {
		return true
	}

	t := rr.Header().Rrtype
	rrs, ok := e.m[t]
	if !ok {
		return
	}

	for i, er := range rrs {
		if equalRdata(er, rr) {
			rrs = removeFromSlice(rrs, i)
			e.m[t] = rrs
			empty = len(rrs) == 0
			if empty {
				delete(e.m, t)
			}
			return
		}
	}
	return
}

// Less is a tree helper function that calls less.
func Less(a *Elem, name string) int { return less(name, a.Name()) }

// Assuming the same type and name this will check if the rdata is equal as well.
func equalRdata(a, b dns.RR) bool {
	switch x := a.(type) {
	// TODO(miek): more types, i.e. all types. + tests for this.
	case *dns.A:
		return x.A.Equal(b.(*dns.A).A)
	case *dns.AAAA:
		return x.AAAA.Equal(b.(*dns.AAAA).AAAA)
	case *dns.MX:
		if x.Mx == b.(*dns.MX).Mx && x.Preference == b.(*dns.MX).Preference {
			return true
		}
	}
	return false
}

// removeFromSlice removes index i from the slice.
func removeFromSlice(rrs []dns.RR, i int) []dns.RR {
	if i >= len(rrs) {
		return rrs
	}
	rrs = append(rrs[:i], rrs[i+1:]...)
	return rrs
}
