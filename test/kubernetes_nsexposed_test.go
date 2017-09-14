// +build k8s

package test

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

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
			test.A("svc-c.test-2.svc.cluster.local.      303    IN      A       10.0.0.120"),
		},
	},
}

func TestKubernetesNSExposed(t *testing.T) {
	corefile :=
		`.:0 {
    kubernetes cluster.local {
                endpoint http://localhost:8080
    }
`

	server, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer server.Stop()

	// Work-around for timing condition that results in no-data being returned in test environment.
	time.Sleep(3 * time.Second)

	for _, tc := range dnsTestCasesAllNSExposed {

		c := new(dns.Client)
		m := tc.Msg()

		res, _, err := c.Exchange(m, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}

		test.SortAndCheck(t, res, tc)
	}
}
