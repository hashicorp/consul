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

// Types returns the types of the records in e. The returned list is not sorted.
func (e *Elem) Types() []uint16 {
	t := make([]uint16, len(e.m))
	i := 0
	for ty := range e.m {
		t[i] = ty
		i++
	}
	return t
}

// Type returns the RRs with type qtype from e.
func (e *Elem) Type(qtype uint16) []dns.RR { return e.m[qtype] }

// TypeForWildcard returns the RRs with type qtype from e. The ownername returned is set to qname.
func (e *Elem) TypeForWildcard(qtype uint16, qname string) []dns.RR {
	rrs := e.m[qtype]

	if rrs == nil {
		return nil
	}

	copied := make([]dns.RR, len(rrs))
	for i := range rrs {
		copied[i] = dns.Copy(rrs[i])
		copied[i].Header().Name = qname
	}
	return copied
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

// Empty returns true is e does not contain any RRs, i.e. is an empty-non-terminal.
func (e *Elem) Empty() bool { return len(e.m) == 0 }

// Insert inserts rr into e. If rr is equal to existing RRs, the RR will be added anyway.
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

	rrs = append(rrs, rr)
	e.m[t] = rrs
}

// Delete removes all RRs of type rr.Header().Rrtype from e.
func (e *Elem) Delete(rr dns.RR) {
	if e.m == nil {
		return
	}

	t := rr.Header().Rrtype
	delete(e.m, t)
}

// Less is a tree helper function that calls less.
func Less(a *Elem, name string) int { return less(name, a.Name()) }
