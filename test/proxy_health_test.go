package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestProxyThreeWay(t *testing.T) {
	// Run 3 CoreDNS server, 2 upstream ones and a proxy. 1 Upstream is unhealthy after 1 query, but after
	// that we should still be able to send to the other one.

	// Backend CoreDNS's.
	corefileUp1 := `example.org:0 {
		erratic {
			drop 2
		}
	}
`

	up1, err := CoreDNSServer(corefileUp1)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer up1.Stop()

	corefileUp2 := `example.org:0 {
		whoami
	}
`

	up2, err := CoreDNSServer(corefileUp2)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer up2.Stop()

	addr1, _ := CoreDNSServerPorts(up1, 0)
	if addr1 == "" {
		t.Fatalf("Could not get UDP listening port")
	}
	addr2, _ := CoreDNSServerPorts(up2, 0)
	if addr2 == "" {
		t.Fatalf("Could not get UDP listening port")
	}

	// Proxying CoreDNS.
	corefileProxy := `example.org:0 {
		forward . ` + addr1 + " " + addr2 + ` {
			max_fails 1
		}
	}`

	prx, err := CoreDNSServer(corefileProxy)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer prx.Stop()
	addr, _ := CoreDNSServerPorts(prx, 0)
	if addr == "" {
		t.Fatalf("Could not get UDP listening port")
	}

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)

	for i := 0; i < 10; i++ {
		r, err := dns.Exchange(m, addr)
		if err != nil {
			continue
		}
		// We would previously get SERVFAIL, so just getting answers here
		// is a good sign. The actual timeouts are handled in the err != nil case
		// above.
		if r.Rcode != dns.RcodeSuccess {
			t.Fatalf("Expected success rcode, got %d", r.Rcode)
		}
	}
}
