package federation

import (
	"testing"

	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestIsNameFederation(t *testing.T) {
	tests := []struct {
		fed          string
		qname        string
		expectedZone string
	}{
		{"prod", "nginx.mynamespace.prod.svc.example.com.", "nginx.mynamespace.svc.example.com."},
		{"prod", "nginx.mynamespace.staging.svc.example.com.", ""},
		{"prod", "nginx.mynamespace.example.com.", ""},
		{"prod", "example.com.", ""},
		{"prod", "com.", ""},
	}

	fed := New()
	for i, tc := range tests {
		fed.f[tc.fed] = "test-name"
		if x, _ := fed.isNameFederation(tc.qname, "example.com."); x != tc.expectedZone {
			t.Errorf("Test %d, failed to get zone, expected %s, got %s", i, tc.expectedZone, x)
		}
	}
}

func TestFederationKubernetes(t *testing.T) {
	tests := []test.Case{
		{
			// service exists so we return the IP address associated with it.
			Qname: "svc1.testns.prod.svc.cluster.local.", Qtype: dns.TypeA,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.A("svc1.testns.prod.svc.cluster.local.      303       IN      A       10.0.0.1"),
			},
		},
		{
			// service does not exist, do the federation dance.
			Qname: "svc0.testns.prod.svc.cluster.local.", Qtype: dns.TypeA,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.CNAME("svc0.testns.prod.svc.cluster.local.  303       IN      CNAME   svc0.testns.prod.svc.fd-az.fd-r.federal.example."),
			},
		},
	}

	k := kubernetes.New([]string{"cluster.local."})
	k.APIConn = &APIConnFederationTest{}

	fed := New()
	fed.zones = []string{"cluster.local."}
	fed.Federations = k.Federations
	fed.Next = k
	fed.f = map[string]string{
		"prod": "federal.example.",
	}

	ctx := context.TODO()
	for i, tc := range tests {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fed.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Test %d, expected no error, got %v\n", i, err)
			return
		}

		resp := rec.Msg
		test.SortAndCheck(t, resp, tc)
	}
}
