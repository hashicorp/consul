// +build k8s

package test

import (
	"testing"
	"time"

	"github.com/miekg/coredns/middleware/test"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

// Test data
// TODO: Fix the actual RR values

var dnsTestCases = []test.Case{
	{
		Qname: "mynginx.demo.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
		},
	},
	{
		Qname: "bogusservice.demo.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "mynginx.*.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
		},
	},
	{
		Qname: "mynginx.any.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
		},
	},
	{
		Qname: "bogusservice.*.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "bogusservice.any.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.demo.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
			test.A("webserver.demo.svc.coredns.local.      1800    IN      A       10.3.0.20"),
		},
	},
	{
		Qname: "any.demo.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
			test.A("webserver.demo.svc.coredns.local.      1800    IN      A       10.3.0.20"),
		},
	},
	{
		Qname: "any.test.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.test.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.*.svc.coredns.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
			test.A("webserver.demo.svc.coredns.local.      1800    IN      A       10.3.0.20"),
		},
	},
	//TODO: Fix below to all use test.SRV not test.A!
	{
		Qname: "mynginx.demo.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
		},
	},
	{
		Qname: "bogusservice.demo.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "mynginx.*.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
		},
	},
	{
		Qname: "mynginx.any.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
		},
	},
	{
		Qname: "bogusservice.*.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "bogusservice.any.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.demo.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
			test.A("webserver.demo.svc.coredns.local.      1800    IN      A       10.3.0.20"),
		},
	},
	{
		Qname: "any.demo.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
			test.A("webserver.demo.svc.coredns.local.      1800    IN      A       10.3.0.20"),
		},
	},
	{
		Qname: "any.test.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.test.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
	{
		Qname: "*.*.svc.coredns.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("mynginx.demo.svc.coredns.local.      1800    IN      A       10.3.0.10"),
			test.A("webserver.demo.svc.coredns.local.      1800    IN      A       10.3.0.20"),
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

func TestKubernetesIntegration(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes coredns.local {
                endpoint http://localhost:8080
		#endpoint https://kubernetes/ admin.pem admin-key.pem ca.pem
		#endpoint https://kubernetes/ 
		#tls k8s_auth/client2.crt k8s_auth/client2.key k8s_auth/ca2.crt
		namespaces demo
    }
`
	server, udp := createTestServer(t, corefile)
	defer server.Stop()

	// Work-around for timing condition that results in no-data being returned in
	// test environment.
	time.Sleep(5 * time.Second)

	for _, tc := range dnsTestCases {
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
