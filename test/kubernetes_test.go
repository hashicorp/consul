// +build k8sIntegration

package test

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

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

// checkKubernetesRunning performs a basic
func checkKubernetesRunning() bool {
	_, err := http.Get("http://localhost:8080/api/v1")
	return err == nil
}

func TestK8sIntegration(t *testing.T) {
	t.Log("   === RUN testLookupA")
	testLookupA(t)
	t.Log("   === RUN testLookupSRV")
	testLookupSRV(t)
}

func testLookupA(t *testing.T) {
	if !checkKubernetesRunning() {
		t.Skip("Skipping Kubernetes Integration tests. Kubernetes is not running")
	}

	// Note: Use different port to avoid conflict with servers used in other tests.
	coreFile :=
		`.:2053 {
    kubernetes coredns.local {
		endpoint http://localhost:8080
		namespaces demo
    }
`

	server, _, udp, err := Server(t, coreFile)
	if err != nil {
		t.Fatal("Could not get server: %s", err)
	}
	defer server.Stop()

	log.SetOutput(ioutil.Discard)

	for _, testData := range testdataLookupA {
		t.Logf("[log] Testing query string: '%v'\n", testData.Query)
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
	if !checkKubernetesRunning() {
		t.Skip("Skipping Kubernetes Integration tests. Kubernetes is not running")
	}

	// Note: Use different port to avoid conflict with servers used in other tests.
	coreFile :=
		`.:2054 {
    kubernetes coredns.local {
		endpoint http://localhost:8080
		namespaces demo
    }
`

	server, _, udp, err := Server(t, coreFile)
	if err != nil {
		t.Fatal("Could not get server: %s", err)
	}
	defer server.Stop()

	log.SetOutput(ioutil.Discard)

	// TODO: Add checks for A records in additional section

	for _, testData := range testdataLookupSRV {
		t.Logf("[log] Testing query string: '%v'\n", testData.Query)
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
			fmt.Printf("RR: %v\n", a)
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
