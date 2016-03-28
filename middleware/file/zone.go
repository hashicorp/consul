package file

import (
	"github.com/miekg/coredns/middleware/file/tree"

	"github.com/miekg/dns"
)

type Zone struct {
	SOA  *dns.SOA
	SIG  []*dns.RRSIG
	name string
	*tree.Tree
}

func NewZone(name string) *Zone {
	return &Zone{name: dns.Fqdn(name), Tree: &tree.Tree{}}
}

func (z *Zone) Insert(r dns.RR) {
	z.Tree.Insert(r)
}

func (z *Zone) Delete(r dns.RR) {
	z.Tree.Delete(r)
}
