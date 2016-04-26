package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

func BenchmarkLookupBalanceRewriteCache(b *testing.B) {
	t := new(testing.T)
	name, rm, err := test.TempFile(t, ".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to created zone: %s", err)
	}
	defer rm()

	corefile := `example.org:0 {
    file ` + name + `
    rewrite ANY HINFO
    loadbalance
}
`
	ex, _, udp, err := Server(t, corefile)
	if err != nil {
		t.Fatalf("Could get server: %s", err)
	}
	defer ex.Stop()

	log.SetOutput(ioutil.Discard)
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Exchange(m, udp)
	}
}
