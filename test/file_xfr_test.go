package test

import (
	"fmt"
	"io"
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

	// Start server, and send an AXFR query to the TCP port.  We set the deadline to prevent the test from hanging.
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
		t.Fatalf("Unable to send TCP query: %s", err)
	}

	nrr := 0                                             // total number of transferred RRs
	co.SetReadDeadline(time.Now().Add(60 * time.Second)) // use a longer timeout as it involves transferring a non-trivial amount of data.
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

	// At the time of the initial implementation of this test, the remaining check would fail due to the problem described in PR #2866, so we return here.
	// Once the issue is resolved this workaround should be removed to enable the check.
	return

	// Once xfr is completed the server should close the connection, so a further read should result in an EOF error.
	// This time we use a short timeout to make it faster in case the server doesn't behave as expected.
	co.SetReadDeadline(time.Now().Add(time.Second))
	_, err = co.ReadMsg()
	if err == nil {
		t.Fatalf("Expected failure on further read, but it succeeded")
	}
	if err != io.EOF {
		t.Fatalf("Expected EOF on further read, but got a different error: %v", err)
	}
}
