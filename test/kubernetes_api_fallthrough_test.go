// +build k8s

package test

import (
	"testing"
	"time"

	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
)

func TestKubernetesAPIFallthrough(t *testing.T) {
	tests := []test.Case{
		{
			Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeA,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			},
		},
	}

	corefile :=
		`.:0 {
    kubernetes cluster.local {
        endpoint http://nonexistance:8080,http://invalidip:8080,http://localhost:8080
        namespaces test-1
        pods disabled
    }`

	server, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer server.Stop()

	// Work-around for timing condition that results in no-data being returned in test environment.
	time.Sleep(3 * time.Second)

	for _, tc := range tests {

		c := new(dns.Client)
		m := tc.Msg()

		res, _, err := c.Exchange(m, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}
		test.SortAndCheck(t, res, tc)
	}
}
