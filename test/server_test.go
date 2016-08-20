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
		t.Fatalf("could not get CoreDNS serving instance: %s", err)
	}

	udpChaos, tcpChaos := CoreDNSServerPorts(chaos, 0)
	defer chaos.Stop()

	corefileProxy := `.:0 {
		proxy . ` + udpChaos + `
}
`
	proxy, err := CoreDNSServer(corefileProxy)
	if err != nil {
		t.Fatalf("could not get CoreDNS serving instance")
	}

	udp, _ := CoreDNSServerPorts(proxy, 0)
	defer proxy.Stop()

	chaosTest(t, udpChaos, "udp")
	chaosTest(t, tcpChaos, "tcp")

	chaosTest(t, udp, "udp")
	// chaosTest(t, tcp, "tcp"), commented out because we use the original transport to reach the
	// proxy and we only forward to the udp port.
}

func chaosTest(t *testing.T, server, net string) {
	m := Msg("version.bind.", dns.TypeTXT, nil)
	m.Question[0].Qclass = dns.ClassCHAOS

	r, err := Exchange(m, server, net)
	if err != nil {
		t.Fatalf("Could not send message: %s", err)
	}
	if r.Rcode != dns.RcodeSuccess || len(r.Answer) == 0 {
		t.Fatalf("Expected successful reply on %s, got %s", net, dns.RcodeToString[r.Rcode])
	}
	if r.Answer[0].String() != `version.bind.	0	CH	TXT	"CoreDNS-001"` {
		t.Fatalf("Expected version.bind. reply, got %s", r.Answer[0].String())
	}
}
