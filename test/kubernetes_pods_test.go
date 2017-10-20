// +build k8s

package test

import (
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

var dnsTestCasesPodsInsecure = []test.Case{
	{
		Qname: "10-20-0-101.test-1.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("10-20-0-101.test-1.pod.cluster.local. 303 IN A    10.20.0.101"),
		},
	},
	{
		Qname: "10-20-0-101.test-X.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502307903 7200 1800 86400 60"),
		},
	},
}

func TestKubernetesPodsInsecure(t *testing.T) {
	corefile := `.:0 {
kubernetes cluster.local 0.0.10.in-addr.arpa {
	endpoint http://localhost:8080
	namespaces test-1
	pods insecure
}
`

	server, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer server.Stop()

	for _, tc := range dnsTestCasesPodsInsecure {

		c := new(dns.Client)
		m := tc.Msg()

		res, _, err := c.Exchange(m, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}

		test.SortAndCheck(t, res, tc)
	}
}

var dnsTestCasesPodsVerified = []test.Case{
	{
		Qname: "10-20-0-101.test-1.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502308197 7200 1800 86400 60"),
		},
	},
	{
		Qname: "10-20-0-101.test-X.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502307960 7200 1800 86400 60"),
		},
	},
}

func TestKubernetesPodsVerified(t *testing.T) {
	corefile := `.:0 {
    kubernetes cluster.local 0.0.10.in-addr.arpa {
                endpoint http://localhost:8080
                namespaces test-1
                pods verified
    }
`

	server, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer server.Stop()

	for _, tc := range dnsTestCasesPodsVerified {

		c := new(dns.Client)
		m := tc.Msg()

		res, _, err := c.Exchange(m, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}

		test.SortAndCheck(t, res, tc)
	}
}
