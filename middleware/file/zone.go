package file

import (
	"sync"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file/tree"

	"github.com/miekg/dns"
)

type Zone struct {
	SOA  *dns.SOA
	SIG  []dns.RR
	name string
	*tree.Tree

	TransferTo   []string
	StartupOnce  sync.Once
	TransferFrom []string
	Expired      *bool
}

// NewZone returns a new zone.
func NewZone(name string) *Zone {
	z := &Zone{name: dns.Fqdn(name), Tree: &tree.Tree{}, Expired: new(bool)}
	*z.Expired = false
	return z
}

// Copy copies a zone *without* copying the zone's content. It is not a deep copy.
func (z *Zone) Copy() *Zone {
	z1 := NewZone(z.name)
	z1.TransferTo = z.TransferTo
	z1.TransferFrom = z.TransferFrom
	z1.Expired = z.Expired
	z1.SOA = z.SOA
	z1.SIG = z.SIG
	return z1
}

// Insert inserts r into z.
func (z *Zone) Insert(r dns.RR) { z.Tree.Insert(r) }

// Delete deletes r from z.
func (z *Zone) Delete(r dns.RR) { z.Tree.Delete(r) }

// TransferAllowed checks if incoming request for transferring the zone is allowed according to the ACLs.
func (z *Zone) TransferAllowed(state middleware.State) bool {
	for _, t := range z.TransferTo {
		if t == "*" {
			return true
		}
	}
	// TODO(miek): future matching against IP/CIDR notations
	return false
}

// All returns all records from the zone, the first record will be the SOA record,
// otionally followed by all RRSIG(SOA)s.
func (z *Zone) All() []dns.RR {
	records := []dns.RR{}
	allNodes := z.Tree.All()
	for _, a := range allNodes {
		records = append(records, a.All()...)
	}

	if len(z.SIG) > 0 {
		records = append(z.SIG, records...)
	}
	return append([]dns.RR{z.SOA}, records...)
}
