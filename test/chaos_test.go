package test

import (
	"testing"

	// Plug in CoreDNS, needed for AppVersion and AppName in this test.
	_ "github.com/coredns/coredns/coremain"

	"github.com/caddyserver/caddy"
	"github.com/miekg/dns"
)

func TestChaos(t *testing.T) {
	corefile := `.:0 {
		chaos
}
`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("version.bind.", dns.TypeTXT)
	m.Question[0].Qclass = dns.ClassCHAOS

	resp, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %v", err)
	}
	chTxt := resp.Answer[0].(*dns.TXT).Txt[0]
	version := caddy.AppName + "-" + caddy.AppVersion
	if chTxt != version {
		t.Fatalf("Expected version to bo %s, got %s", version, chTxt)
	}
}
