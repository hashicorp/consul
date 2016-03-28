package file

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/file/tree"

	"github.com/miekg/dns"
)

type Transfer struct {
	Out bool
	In  bool
	// more later
}

type Zone struct {
	SOA  *dns.SOA
	SIG  []dns.RR
	name string
	*tree.Tree
	Masters  []string
	Transfer *Transfer
}

func NewZone(name string) *Zone {
	return &Zone{name: dns.Fqdn(name), Tree: &tree.Tree{}, Transfer: &Transfer{}}
}

func (z *Zone) Insert(r dns.RR) {
	z.Tree.Insert(r)
}

func (z *Zone) Delete(r dns.RR) {
	z.Tree.Delete(r)
}

// It the transfer request allowed.
func (z *Zone) TransferAllowed(state middleware.State) bool {
	if z.Transfer == nil {
		return false
	}
	return z.Transfer.Out
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

// Apex function?
