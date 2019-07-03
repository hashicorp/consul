package test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestLargeAXFR(t *testing.T) {
	// Build a large zone in text format.  It contains 64K AAAA RRs.
	var sb strings.Builder
	const numAAAAs = 65536
	sb.WriteString("example.com. IN SOA . . 1 60 60 60 60\n")
	sb.WriteString("example.com. IN NS ns.example.\n")
	for i := 0; i < numAAAAs; i++ {
		sb.WriteString(fmt.Sprintf("%d.example.com. IN AAAA 2001:db8::1\n", i))
	}

	// Setup the zone file and CoreDNS to serve the zone, allowing zone transfer
	name, rm, err := test.TempFile(".", sb.String())
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	corefile := `example.com:0 {
       file ` + name + ` {
           transfer to *
       }
}
`
	// Start server, and send an AXFR query to the TCP port. We set the deadline to prevent the test from hanging.
	i, _, tcp, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeAXFR)
	co, err := dns.DialTimeout("tcp", tcp, 5*time.Second)
	if err != nil {
		t.Fatalf("Expected to establish TCP connection, but didn't: %s", err)
	}
	defer co.Close()
	co.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err = co.WriteMsg(m)
	if err != nil {
		t.Fatalf("Unable to send AXFR/TCP query: %s", err)
	}

	// Then send another query on the same connection.  We use this to confirm that multiple outstanding queries won't cause a race.
	m.SetQuestion("0.example.com.", dns.TypeAAAA)
	err = co.WriteMsg(m)
	if err != nil {
		t.Fatalf("Unable to send AAAA/TCP query: %s", err)
	}

	// The AXFR query should be responded first.
	nrr := 0 // total number of transferred RRs
	for {
		resp, err := co.ReadMsg()
		if err != nil {
			t.Fatalf("Expected to receive reply, but didn't: %s", err)
		}
		if len(resp.Answer) == 0 {
			continue
		}
		// First RR should be SOA.
		if nrr == 0 && resp.Answer[0].Header().Rrtype != dns.TypeSOA {
			t.Fatalf("Expected SOA, but got type %d", resp.Answer[0].Header().Rrtype)
		}
		nrr += len(resp.Answer)
		// If we see another SOA at the end of the message, we are done.
		// Note that this check is not enough to detect all invalid responses, but checking those is not the purpose of this test.
		if nrr > 1 && resp.Answer[len(resp.Answer)-1].Header().Rrtype == dns.TypeSOA {
			break
		}
	}
	// On successful completion, 2 SOA, 1 NS, and all AAAAs should have been transferred.
	if nrr != numAAAAs+3 {
		t.Fatalf("Got an unexpected number of RRs: %d", nrr)
	}

	// The file plugin shouldn't hijack or (yet) close the connection, so the second query should also be responded.
	resp, err := co.ReadMsg()
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}
	if len(resp.Answer) < 1 {
		t.Fatalf("Expected a non-empty answer, but it was empty")
	}
	if resp.Answer[len(resp.Answer)-1].Header().Rrtype != dns.TypeAAAA {
		t.Fatalf("Expected a AAAA answer, but it wasn't: type %d", resp.Answer[len(resp.Answer)-1].Header().Rrtype)
	}
}
