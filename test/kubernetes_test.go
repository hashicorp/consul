// +build k8s

package test

import (
	"os"
	"testing"
	"time"

	"github.com/coredns/coredns/middleware/test"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

// Test data
// TODO: Fix the actual RR values

var dnsTestCases = []test.Case{
	{
		Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "bogusservice.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "svc-1-a.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "svc-1-a.any.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "bogusservice.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "bogusservice.any.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("svc-1-b.test-1.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("svc-c.test-1.svc.cluster.local.        303    IN      A       10.0.0.115"),
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.5"),
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.6"),
		},
	},
	{
		Qname: "any.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("svc-1-b.test-1.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("svc-c.test-1.svc.cluster.local.        303    IN      A       10.0.0.115"),
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.5"),
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.6"),
		},
	},
	{
		Qname: "any.test-2.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.test-2.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("svc-1-b.test-1.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("svc-c.test-1.svc.cluster.local.        303    IN      A       10.0.0.115"),
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.5"),
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.6"),
		},
	},
	{
		Qname: "headless-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.5"),
			test.A("headless-svc.test-1.svc.cluster.local.      303    IN      A       172.17.0.6"),
		},
	},
	//TODO: Fix below to all use test.SRV not test.A!
	{
		Qname: "*._TcP.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_https._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 443 svc-1-a.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "*.*.bogusservice.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.any.svc-1-a.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_https._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 443 svc-1-a.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "ANY.*.svc-1-a.any.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_https._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 443 svc-1-a.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "*.*.bogusservice.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.*.bogusservice.any.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "_c-port._UDP.*.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_c-port._udp.svc-c.test-1.svc.cluster.local.      303    IN    SRV 10 100 1234 svc-c.test-1.svc.cluster.local."),
			test.SRV("_c-port._udp.headless-svc.test-1.svc.cluster.local.      303    IN    SRV 10 100 1234 172-17-0-5.headless-svc.test-1.svc.cluster.local."),
			test.SRV("_c-port._udp.headless-svc.test-1.svc.cluster.local.      303    IN    SRV 10 100 1234 172-17-0-6.headless-svc.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "*._tcp.any.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_https._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_http._tcp.svc-1-b.test-1.svc.cluster.local.      303    IN    SRV 10 100 80 svc-1-b.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "*.*.any.test-2.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.*.*.test-2.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "_http._tcp.*.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc-1-a.test-1.svc.cluster.local.      303    IN    SRV 10 100 80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_http._tcp.svc-1-b.test-1.svc.cluster.local.      303    IN    SRV 10 100 80 svc-1-b.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "*.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*._not-udp-or-tcp.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "10-20-0-101.test-1.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeServerFailure,
		Answer: []dns.RR{},
	},
	{
		Qname: "123.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode:  dns.RcodeSuccess,
		Answer: []dns.RR{},
	},
	{
		Qname: "100.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("100.0.0.10.in-addr.arpa.      303    IN      PTR       svc-1-a.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "115.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("115.0.0.10.in-addr.arpa.      303    IN      PTR       svc-c.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "dns-version.cluster.local.", Qtype: dns.TypeTXT,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.TXT("dns-version.cluster.local. 28800 IN TXT \"1.0.0\""),
		},
	},
	{
		Qname: "next-in-chain.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("next-in-chain.              0       IN      A       192.0.2.53"),
		},
	},
	{
		Qname: "cluster.local.", Qtype: dns.TypeNS,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.NS("cluster.local.          0       IN      NS      kubernetes.default.svc.cluster.local."),
		},
	},
}

var dnsTestCasesPodsInsecure = []test.Case{
	{
		Qname: "10-20-0-101.test-1.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("10-20-0-101.test-1.pod.cluster.local. 0 IN A    10.20.0.101"),
		},
	},
	{
		Qname: "10-20-0-101.test-X.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
}

var dnsTestCasesPodsVerified = []test.Case{
	{
		Qname: "10-20-0-101.test-1.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "10-20-0-101.test-X.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
}

var dnsTestCasesCidrReverseZone = []test.Case{
	{
		Qname: "123.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode:  dns.RcodeSuccess,
		Answer: []dns.RR{},
	},
	{
		Qname: "100.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("100.0.0.10.in-addr.arpa.      303    IN      PTR       svc-1-a.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "110.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("115.0.0.10.in-addr.arpa.      303    IN      PTR       svc-1-b.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "115.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("115.0.0.10.in-addr.arpa.      303    IN      PTR       svc-c.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "next-in-chain.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("next-in-chain.              0       IN      A       192.0.2.53"),
		},
	},
}

var dnsTestCasesPartialCidrReverseZone = []test.Case{
	{
		// In exposed range, record not present = OK + No data
		Qname: "99.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode:  dns.RcodeSuccess,
		Answer: []dns.RR{},
	},
	{
		// In exposed range, record present = OK + Data
		Qname: "100.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("100.0.0.10.in-addr.arpa.      303    IN      PTR       svc-1-a.test-1.svc.cluster.local."),
		},
	},
	{
		// In exposed range, record present = OK + Data
		Qname: "110.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("115.0.0.10.in-addr.arpa.      303    IN      PTR       svc-1-b.test-1.svc.cluster.local."),
		},
	},
	{
		// Out of exposed range, record present = pass to next middleware (not existing in test) = FAIL
		Qname: "115.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode:  dns.RcodeServerFailure,
		Answer: []dns.RR{},
	},
	{
		Qname: "next-in-chain.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("next-in-chain.              0       IN      A       192.0.2.53"),
		},
	},
}

var dnsTestCasesAllNSExposed = []test.Case{
	{
		Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "svc-c.test-2.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-c.test-1.svc.cluster.local.      303    IN      A       10.0.0.120"),
		},
	},
	{
		Qname: "123.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode:  dns.RcodeSuccess,
		Answer: []dns.RR{},
	},
	{
		Qname: "100.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("100.0.0.10.in-addr.arpa.      303    IN      PTR       svc-1-a.test-1.svc.cluster.local."),
		},
	},
	{
		Qname: "120.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("120.0.0.10.in-addr.arpa.      303    IN      PTR       svc-c.test-2.svc.cluster.local."),
		},
	},
}

var dnsTestCasesFallthrough = []test.Case{
	{
		Qname: "f.b.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("f.b.svc.cluster.local.      303    IN      A       10.10.10.11"),
		},
	},
	{
		Qname: "foo.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("foo.cluster.local.      303    IN      A       10.10.10.10"),
		},
	},
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

func doIntegrationTests(t *testing.T, corefile string, testCases []test.Case) {
	server, udp := createTestServer(t, corefile)
	defer server.Stop()

	// Work-around for timing condition that results in no-data being returned in
	// test environment.
	time.Sleep(1 * time.Second)

	for _, tc := range testCases {

		dnsClient := new(dns.Client)
		dnsMessage := new(dns.Msg)

		dnsMessage.SetQuestion(tc.Qname, tc.Qtype)

		res, _, err := dnsClient.Exchange(dnsMessage, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}

		// check the answer
		if res.Rcode != tc.Rcode {
			t.Errorf("Expected rcode %d but got %d for query %s, %d", tc.Rcode, res.Rcode, tc.Qname, tc.Qtype)
		}

		if len(res.Answer) != len(tc.Answer) {
			t.Errorf("Expected %d answers but got %d for query %s, %d", len(tc.Answer), len(res.Answer), tc.Qname, tc.Qtype)
		}

		//TODO: Check the actual RR values
	}
}

func TestKubernetesIntegration(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes cluster.local 0.0.10.in-addr.arpa {
                endpoint http://localhost:8080
		namespaces test-1
		pods disabled
    }
	erratic . {
		drop 0
	}
`
	doIntegrationTests(t, corefile, dnsTestCases)
}

func TestKubernetesIntegrationPodsInsecure(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes cluster.local 0.0.10.in-addr.arpa {
                endpoint http://localhost:8080
		namespaces test-1
		pods insecure
    }
`
	doIntegrationTests(t, corefile, dnsTestCasesPodsInsecure)
}

func TestKubernetesIntegrationPodsVerified(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes cluster.local 0.0.10.in-addr.arpa {
                endpoint http://localhost:8080
                namespaces test-1
                pods verified
    }
`
	doIntegrationTests(t, corefile, dnsTestCasesPodsVerified)
}

func TestKubernetesIntegrationCidrReverseZone(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes cluster.local {
                endpoint http://localhost:8080
                namespaces test-1
				cidrs 10.0.0.0/24				
    }
	erratic . {
		drop 0
	}
`
	doIntegrationTests(t, corefile, dnsTestCasesCidrReverseZone)
}

func TestKubernetesIntegrationPartialCidrReverseZone(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes cluster.local {
                endpoint http://localhost:8080
                namespaces test-1
				cidrs 10.0.0.96/28 10.0.0.120/32
    }
	erratic . {
		drop 0
	}
`
	doIntegrationTests(t, corefile, dnsTestCasesPartialCidrReverseZone)
}

func TestKubernetesIntegrationAllNSExposed(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes cluster.local {
                endpoint http://localhost:8080
				cidrs 10.0.0.0/24
    }
`
	doIntegrationTests(t, corefile, dnsTestCasesAllNSExposed)
}

func TestKubernetesIntegrationFallthrough(t *testing.T) {
	dbfile, rmFunc, err := TempFile(os.TempDir(), clusterLocal)
	if err != nil {
		t.Fatalf("Could not create TempFile for fallthrough: %s", err)
	}
	defer rmFunc()
	corefile :=
		`.:0 {
    file ` + dbfile + ` cluster.local
    kubernetes cluster.local {
                endpoint http://localhost:8080
		cidrs 10.0.0.0/24
		namespaces test-1
		fallthrough
    }
    erratic {
	drop 0
    }
`
	cases := append(dnsTestCases, dnsTestCasesFallthrough...)
	doIntegrationTests(t, corefile, cases)
}

const clusterLocal = `; cluster.local test file for fallthrough
cluster.local.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
cluster.local.		IN	NS	b.iana-servers.net.
cluster.local.		IN	NS	a.iana-servers.net.
cluster.local.		IN	A	127.0.0.1
cluster.local.		IN	A	127.0.0.2
foo.cluster.local.      IN      A	10.10.10.10
f.b.svc.cluster.local.  IN      A	10.10.10.11
*.w.cluster.local.      IN      TXT     "Wildcard"
a.b.svc.cluster.local.  IN      TXT     "Not a wildcard"
cname.cluster.local.    IN      CNAME   www.example.net.

service.namespace.svc.cluster.local.    IN      SRV     8080 10 10 cluster.local.
`
