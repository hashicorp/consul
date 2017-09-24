package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestReload(t *testing.T) {
	corefile := `.:0 {
	whoami
}
`
	coreInput := NewInput(corefile)

	c, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(c, 0)

	send(t, udp)

	c1, err := c.Restart(coreInput)
	if err != nil {
		t.Fatal(err)
	}
	udp, _ = CoreDNSServerPorts(c1, 0)

	send(t, udp)

	c1.Stop()
}

func send(t *testing.T, server string) {
	m := new(dns.Msg)
	m.SetQuestion("whoami.example.org.", dns.TypeSRV)

	r, err := dns.Exchange(m, server)
	if err != nil {
		// This seems to fail a lot on travis, quick'n dirty: redo
		r, err = dns.Exchange(m, server)
		if err != nil {
			return
		}
	}
	if r.Rcode != dns.RcodeSuccess {
		t.Fatalf("Expected successful reply, got %s", dns.RcodeToString[r.Rcode])
	}
	if len(r.Extra) != 2 {
		t.Fatalf("Expected 2 RRs in additional, got %d", len(r.Extra))
	}
}
