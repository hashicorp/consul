// +build k8s

package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/miekg/coredns/middleware/kubernetes/k8stest"

	"github.com/miekg/dns"
)

// Test data for A records
var testdataLookupA = []struct {
	Query            string
	TotalAnswerCount int
	ARecordCount     int
}{
	// Matching queries
	{"mynginx.demo.coredns.local.", 1, 1}, // One A record, should exist

	// Failure queries
	{"mynginx.test.coredns.local.", 0, 0},                     // One A record, is not exposed
	{"someservicethatdoesnotexist.demo.coredns.local.", 0, 0}, // Record does not exist

	// Namespace wildcards
	{"mynginx.*.coredns.local.", 1, 1},                       // One A record, via wildcard namespace
	{"mynginx.any.coredns.local.", 1, 1},                     // One A record, via wildcard namespace
	{"someservicethatdoesnotexist.*.coredns.local.", 0, 0},   // Record does not exist with wildcard for namespace
	{"someservicethatdoesnotexist.any.coredns.local.", 0, 0}, // Record does not exist with wildcard for namespace
	{"*.demo.coredns.local.", 2, 2},                          // Two A records, via wildcard
	{"any.demo.coredns.local.", 2, 2},                        // Two A records, via wildcard
	{"*.test.coredns.local.", 0, 0},                          // Two A record, via wildcard that is not exposed
	{"any.test.coredns.local.", 0, 0},                        // Two A record, via wildcard that is not exposed
	{"*.*.coredns.local.", 2, 2},                             // Two A records, via namespace and service wildcard
}

// Test data for SRV records
var testdataLookupSRV = []struct {
	Query            string
	TotalAnswerCount int
	//	ARecordCount     int
	SRVRecordCount int
}{
	// Matching queries
	{"mynginx.demo.coredns.local.", 1, 1}, // One SRV record, should exist

	// Failure queries
	{"mynginx.test.coredns.local.", 0, 0},                     // One SRV record, is not exposed
	{"someservicethatdoesnotexist.demo.coredns.local.", 0, 0}, // Record does not exist

	// Namespace wildcards
	{"mynginx.*.coredns.local.", 1, 1},                       // One SRV record, via wildcard namespace
	{"mynginx.any.coredns.local.", 1, 1},                     // One SRV record, via wildcard namespace
	{"someservicethatdoesnotexist.*.coredns.local.", 0, 0},   // Record does not exist with wildcard for namespace
	{"someservicethatdoesnotexist.any.coredns.local.", 0, 0}, // Record does not exist with wildcard for namespace
	{"*.demo.coredns.local.", 1, 1},                          // One SRV record, via wildcard
	{"any.demo.coredns.local.", 1, 1},                        // One SRV record, via wildcard
	{"*.test.coredns.local.", 0, 0},                          // One SRV record, via wildcard that is not exposed
	{"any.test.coredns.local.", 0, 0},                        // One SRV record, via wildcard that is not exposed
	{"*.*.coredns.local.", 1, 1},                             // One SRV record, via namespace and service wildcard
}

func TestK8sIntegration(t *testing.T) {
	// subtests here (Go 1.7 feature).
	testLookupA(t)
	testLookupSRV(t)
}

func testLookupA(t *testing.T) {
	if !k8stest.CheckKubernetesRunning() {
		t.Skip("Skipping Kubernetes Integration tests. Kubernetes is not running")
	}

	corefile :=
		`.:0 {
    kubernetes coredns.local {
		endpoint http://localhost:8080
		namespaces demo
    }
`

	server, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(server, 0)
	if udp == "" {
		t.Fatalf("could not get udp listening port")
	}
	defer server.Stop()

	log.SetOutput(ioutil.Discard)

	for _, testData := range testdataLookupA {
		dnsClient := new(dns.Client)
		dnsMessage := new(dns.Msg)

		dnsMessage.SetQuestion(testData.Query, dns.TypeA)
		dnsMessage.SetEdns0(4096, true)

		res, _, err := dnsClient.Exchange(dnsMessage, udp)
		if err != nil {
			t.Fatal("Could not send query: %s", err)
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
	if !k8stest.CheckKubernetesRunning() {
		t.Skip("Skipping Kubernetes Integration tests. Kubernetes is not running")
	}

	corefile :=
		`.:0 {
    kubernetes coredns.local {
		endpoint http://localhost:8080
		namespaces demo
    }
`

	server, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("could not get CoreDNS serving instance: %s", err)
	}
	udp, _ := CoreDNSServerPorts(server, 0)
	if udp == "" {
		t.Fatalf("could not get udp listening port")
	}
	defer server.Stop()

	log.SetOutput(ioutil.Discard)

	// TODO: Add checks for A records in additional section

	for _, testData := range testdataLookupSRV {
		dnsClient := new(dns.Client)
		dnsMessage := new(dns.Msg)

		dnsMessage.SetQuestion(testData.Query, dns.TypeSRV)
		dnsMessage.SetEdns0(4096, true)

		res, _, err := dnsClient.Exchange(dnsMessage, udp)
		if err != nil {
			t.Fatal("Could not send query: %s", err)
		}
		// Count SRV records in the answer section
		srvRecordCount := 0
		for _, a := range res.Answer {
			if a.Header().Rrtype == dns.TypeSRV {
				srvRecordCount++
			}
		}

		if srvRecordCount != testData.SRVRecordCount {
			t.Errorf("Expected '%v' SRV records in response. Instead got '%v' SRV records. Test query string: '%v'", testData.SRVRecordCount, srvRecordCount, testData.Query)
		}
		if len(res.Answer) != testData.TotalAnswerCount {
			t.Errorf("Expected '%v' records in answer section. Instead got '%v' records in answer section. Test query string: '%v'", testData.TotalAnswerCount, len(res.Answer), testData.Query)
		}
	}
}
