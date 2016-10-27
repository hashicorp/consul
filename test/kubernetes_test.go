// +build k8s

package test

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

// Test data for A records
var testdataLookupA = []struct {
	Query            string
	TotalAnswerCount int
	ARecordCount     int
}{
	// Matching queries
	{"mynginx.demo.svc.coredns.local.", 1, 1}, // One A record, should exist

	// Failure queries
	{"mynginx.test.svc.coredns.local.", 0, 0},                     // One A record, is not exposed
	{"someservicethatdoesnotexist.demo.svc.coredns.local.", 0, 0}, // Record does not exist

	// Namespace wildcards
	{"mynginx.*.svc.coredns.local.", 1, 1},                       // One A record, via wildcard namespace
	{"mynginx.any.svc.coredns.local.", 1, 1},                     // One A record, via wildcard namespace
	{"someservicethatdoesnotexist.*.svc.coredns.local.", 0, 0},   // Record does not exist with wildcard for namespace
	{"someservicethatdoesnotexist.any.svc.coredns.local.", 0, 0}, // Record does not exist with wildcard for namespace
	{"*.demo.svc.coredns.local.", 2, 2},                          // Two A records, via wildcard
	{"any.demo.svc.coredns.local.", 2, 2},                        // Two A records, via wildcard
	{"*.test.svc.coredns.local.", 0, 0},                          // Two A record, via wildcard that is not exposed
	{"any.test.svc.coredns.local.", 0, 0},                        // Two A record, via wildcard that is not exposed
	{"*.*.svc.coredns.local.", 2, 2},                             // Two A records, via namespace and service wildcard
}

// Test data for SRV records
var testdataLookupSRV = []struct {
	Query            string
	TotalAnswerCount int
	//	ARecordCount     int
	SRVRecordCount int
}{
	// Matching queries
	{"mynginx.demo.svc.coredns.local.", 1, 1}, // One SRV record, should exist

	// Failure queries
	{"mynginx.test.svc.coredns.local.", 0, 0},                     // One SRV record, is not exposed
	{"someservicethatdoesnotexist.demo.svc.coredns.local.", 0, 0}, // Record does not exist

	// Namespace wildcards
	{"mynginx.*.svc.coredns.local.", 1, 1},                       // One SRV record, via wildcard namespace
	{"mynginx.any.svc.coredns.local.", 1, 1},                     // One SRV record, via wildcard namespace
	{"someservicethatdoesnotexist.*.svc.coredns.local.", 0, 0},   // Record does not exist with wildcard for namespace
	{"someservicethatdoesnotexist.any.svc.coredns.local.", 0, 0}, // Record does not exist with wildcard for namespace
	{"*.demo.svc.coredns.local.", 2, 2},                          // Two (mynginx, webserver) SRV record, via wildcard
	{"any.demo.svc.coredns.local.", 2, 2},                        // Two (mynginx, webserver) SRV record, via wildcard
	{"*.test.svc.coredns.local.", 0, 0},                          // One SRV record, via wildcard that is not exposed
	{"any.test.svc.coredns.local.", 0, 0},                        // One SRV record, via wildcard that is not exposed
	{"*.*.svc.coredns.local.", 2, 2},                             // Two SRV record, via namespace and service wildcard
}

func TestKubernetesIntegration(t *testing.T) {

	// t.Skip("Skip Kubernetes Integration tests")
	// subtests here (Go 1.7 feature).
	testLookupA(t)
	testLookupSRV(t)
}

func createTestServer(t *testing.T, corefile string) (*caddy.Instance, string) {
	server, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(server, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}

	return server, udp
}

func testLookupA(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes coredns.local {
                endpoint http://localhost:8080
		namespaces demo
    }

`
	server, udp := createTestServer(t, corefile)
	defer server.Stop()

	log.SetOutput(ioutil.Discard)

	// Work-around for timing condition that results in no-data being returned in
	// test environment.
	time.Sleep(5 * time.Second)

	for _, testData := range testdataLookupA {
		dnsClient := new(dns.Client)
		dnsMessage := new(dns.Msg)

		dnsMessage.SetQuestion(testData.Query, dns.TypeA)
		dnsMessage.SetEdns0(4096, true)

		res, _, err := dnsClient.Exchange(dnsMessage, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}
		// Count A records in the answer section
		ARecordCount := 0
		for _, a := range res.Answer {
			if a.Header().Rrtype == dns.TypeA {
				ARecordCount++
			}
		}

		if ARecordCount != testData.ARecordCount {
			t.Errorf("Expected '%v' A records in response. Instead got '%v' A records. Test query string: '%v'", testData.ARecordCount, ARecordCount, testData.Query)
		}
		if len(res.Answer) != testData.TotalAnswerCount {
			t.Errorf("Expected '%v' records in answer section. Instead got '%v' records in answer section. Test query string: '%v'", testData.TotalAnswerCount, len(res.Answer), testData.Query)
		}
	}
}

func testLookupSRV(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes coredns.local {
                endpoint http://localhost:8080
		namespaces demo
    }
`

	server, udp := createTestServer(t, corefile)
	defer server.Stop()

	log.SetOutput(ioutil.Discard)

	// Work-around for timing condition that results in no-data being returned in
	// test environment.
	time.Sleep(5 * time.Second)

	// TODO: Add checks for A records in additional section

	for _, testData := range testdataLookupSRV {
		dnsClient := new(dns.Client)
		dnsMessage := new(dns.Msg)

		dnsMessage.SetQuestion(testData.Query, dns.TypeSRV)
		dnsMessage.SetEdns0(4096, true)

		res, _, err := dnsClient.Exchange(dnsMessage, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}
		// Count SRV records in the answer section
		srvRecordCount := 0
		for _, a := range res.Answer {
			if a.Header().Rrtype == dns.TypeSRV {
				srvRecordCount++
			}
		}

		if srvRecordCount != testData.SRVRecordCount {
			t.Errorf("Expected '%v' SRV records in response. Instead got '%v' SRV records. Test query string: '%v', res: %v", testData.SRVRecordCount, srvRecordCount, testData.Query, res)
		}
		if len(res.Answer) != testData.TotalAnswerCount {
			t.Errorf("Expected '%v' records in answer section. Instead got '%v' records in answer section. Test query string: '%v', res: %v", testData.TotalAnswerCount, len(res.Answer), testData.Query, res)
		}
	}
}
