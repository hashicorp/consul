package file

import (
	"testing"

	"github.com/miekg/dns"
)

func TestZoneInsert(t *testing.T) {
	z := NewZone("miek.nl")
	rr, _ := dns.NewRR("miek.nl. IN A 127.0.0.1")
	z.Insert(rr)

	t.Logf("%+v\n", z)

	elem := z.Get(rr)
	t.Logf("%+v\n", elem)
	if elem != nil {
		t.Logf("%+v\n", elem.Types(dns.TypeA))
	}
	z.Delete(rr)

	t.Logf("%+v\n", z)

	elem = z.Get(rr)
	t.Logf("%+v\n", elem)
	if elem != nil {
		t.Logf("%+v\n", elem.Types(dns.TypeA))
	}
}
