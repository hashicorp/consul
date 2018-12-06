package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestEDNS0(t *testing.T) {
	corefile := `.:0 {
		whoami
}
`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeSOA)
	m.SetEdns0(4096, true)

	resp, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %v", err)
	}
	opt := resp.Extra[len(resp.Extra)-1]
	if opt.Header().Rrtype != dns.TypeOPT {
		t.Errorf("Last RR must be OPT record")
	}
}
