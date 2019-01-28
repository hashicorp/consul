package test

import (
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestFileUpstream(t *testing.T) {
	name, rm, err := test.TempFile(".", `$ORIGIN example.org.
@	3600 IN	SOA sns.dns.icann.org. noc.dns.icann.org. (
		2017042745 ; serial
		7200       ; refresh (2 hours)
		3600       ; retry (1 hour)
		1209600    ; expire (2 weeks)
		3600       ; minimum (1 hour)
	)

        3600 IN NS a.iana-servers.net.
	3600 IN NS b.iana-servers.net.

www 3600 IN CNAME   www.example.net.
`)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	corefile := `.:0 {
	file ` + name + ` example.org {
	       upstream
	}
	hosts {
               10.0.0.1 www.example.net.
               fallthrough
       }
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("www.example.org.", dns.TypeA)
	m.SetEdns0(4096, true)

	r, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not exchange msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if x := r.Answer[1].(*dns.A).A.String(); x != "10.0.0.1" {
		t.Errorf("Failed to get address for CNAME, expected 10.0.0.1 got %s", x)
	}
}

// TestFileUpstreamAdditional runs two CoreDNS servers that serve example.org and foo.example.org.
// example.org contains a cname to foo.example.org; this should be resolved via upstream.Self.
func TestFileUpstreamAdditional(t *testing.T) {
	name, rm, err := test.TempFile(".", `$ORIGIN example.org.
@	3600 IN	SOA sns.dns.icann.org. noc.dns.icann.org. 2017042745 7200 3600 1209600 3600

	3600 IN NS b.iana-servers.net.

www 3600 IN CNAME   www.foo
`)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	name2, rm2, err2 := test.TempFile(".", `$ORIGIN foo.example.org.
@	3600 IN	SOA sns.dns.icann.org. noc.dns.icann.org. 2017042745 7200 3600 1209600 3600

	3600 IN NS b.iana-servers.net.

www 3600 IN A   127.0.0.53
`)
	if err2 != nil {
		t.Fatalf("Failed to create zone: %s", err2)
	}
	defer rm2()

	corefile := `.:0 {
	file ` + name + ` example.org {
	       upstream
	}
	file ` + name2 + ` foo.example.org {
	       upstream
	}
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("www.example.org.", dns.TypeA)

	r, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not exchange msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if x := len(r.Answer); x != 2 {
		t.Errorf("Expected 2 RR in reply, got %d", x)
	}
	if x := r.Answer[1].(*dns.A).A.String(); x != "127.0.0.53" {
		t.Errorf("Failed to get address for CNAME, expected 127.0.0.53, got %s", x)
	}
}
