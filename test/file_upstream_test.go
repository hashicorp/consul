package test

import (
	"testing"

	"github.com/miekg/dns"
)

// TODO(miek): this test needs to be fleshed out.

func TestFileUpstream(t *testing.T) {
	name, rm, err := TempFile(".", `$ORIGIN example.org.
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
