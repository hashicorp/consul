package test

import (
	"testing"

	"github.com/miekg/dns"
)

// Start 2 tests server, server A will proxy to B, server B is an CH server.
func TestProxyToChaosServer(t *testing.T) {
	corefile := `.:0 {
	chaos CoreDNS-001 miek@miek.nl
}
`
	chaos, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udpChaos, _ := CoreDNSServerPorts(chaos, 0)
	defer chaos.Stop()

	corefileProxy := `.:0 {
		proxy . ` + udpChaos + `
}
`
	proxy, err := CoreDNSServer(corefileProxy)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance")
	}

	udp, _ := CoreDNSServerPorts(proxy, 0)
	defer proxy.Stop()

	chaosTest(t, udpChaos)

	chaosTest(t, udp)
	// chaosTest(t, tcp, "tcp"), commented out because we use the original transport to reach the
	// proxy and we only forward to the udp port.
}

func chaosTest(t *testing.T, server string) {
	m := new(dns.Msg)
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{Qclass: dns.ClassCHAOS, Name: "version.bind.", Qtype: dns.TypeTXT}

	r, err := dns.Exchange(m, server)
	if err != nil {
		t.Fatalf("Could not send message: %s", err)
	}
	if r.Rcode != dns.RcodeSuccess || len(r.Answer) == 0 {
		t.Fatalf("Expected successful reply, got %s", dns.RcodeToString[r.Rcode])
	}
	if r.Answer[0].String() != `version.bind.	0	CH	TXT	"CoreDNS-001"` {
		t.Fatalf("Expected version.bind. reply, got %s", r.Answer[0].String())
	}
}
