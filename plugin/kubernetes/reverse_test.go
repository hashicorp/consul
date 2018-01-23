package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type APIConnReverseTest struct{}

func (APIConnReverseTest) HasSynced() bool                 { return true }
func (APIConnReverseTest) Run()                            { return }
func (APIConnReverseTest) Stop() error                     { return nil }
func (APIConnReverseTest) PodIndex(string) []*api.Pod      { return nil }
func (APIConnReverseTest) SvcIndex(string) []*api.Service  { return nil }
func (APIConnReverseTest) EpIndex(string) []*api.Endpoints { return nil }
func (APIConnReverseTest) EndpointsList() []*api.Endpoints { return nil }
func (APIConnReverseTest) ServiceList() []*api.Service     { return nil }

func (APIConnReverseTest) SvcIndexReverse(ip string) []*api.Service {
	if ip != "192.168.1.100" {
		return nil
	}
	svcs := []*api.Service{
		{
			ObjectMeta: meta.ObjectMeta{
				Name:      "svc1",
				Namespace: "testns",
			},
			Spec: api.ServiceSpec{
				ClusterIP: "192.168.1.100",
				Ports: []api.ServicePort{{
					Name:     "http",
					Protocol: "tcp",
					Port:     80,
				}},
			},
		},
	}
	return svcs
}

func (APIConnReverseTest) EpIndexReverse(ip string) []*api.Endpoints {
	switch ip {
	case "10.0.0.100":
	case "1234:abcd::1":
	case "fd00:77:30::a":
	case "fd00:77:30::2:9ba6":
	default:
		return nil
	}
	eps := []*api.Endpoints{
		{
			Subsets: []api.EndpointSubset{
				{
					Addresses: []api.EndpointAddress{
						{
							IP:       "10.0.0.100",
							Hostname: "ep1a",
						},
						{
							IP:       "1234:abcd::1",
							Hostname: "ep1b",
						},
						{
							IP:       "fd00:77:30::a",
							Hostname: "ip6svc1ex",
						},
						{
							IP:       "fd00:77:30::2:9ba6",
							Hostname: "ip6svc1in",
						},
					},
					Ports: []api.EndpointPort{
						{
							Port:     80,
							Protocol: "tcp",
							Name:     "http",
						},
					},
				},
			},
			ObjectMeta: meta.ObjectMeta{
				Name:      "svc1",
				Namespace: "testns",
			},
		},
	}
	return eps
}

func (APIConnReverseTest) GetNodeByName(name string) (*api.Node, error) {
	return &api.Node{
		ObjectMeta: meta.ObjectMeta{
			Name: "test.node.foo.bar",
		},
	}, nil
}

func (APIConnReverseTest) GetNamespaceByName(name string) (*api.Namespace, error) {
	return &api.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
	}, nil
}

func TestReverse(t *testing.T) {

	k := New([]string{"cluster.local.", "0.10.in-addr.arpa.", "168.192.in-addr.arpa.", "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.d.c.b.a.4.3.2.1.ip6.arpa.", "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.7.7.0.0.0.0.d.f.ip6.arpa."})
	k.APIConn = &APIConnReverseTest{}

	tests := []test.Case{
		{
			Qname: "100.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.PTR("100.0.0.10.in-addr.arpa.      5    IN      PTR       ep1a.svc1.testns.svc.cluster.local."),
			},
		},
		{
			Qname: "100.1.168.192.in-addr.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.PTR("100.1.168.192.in-addr.arpa.     5     IN      PTR       svc1.testns.svc.cluster.local."),
			},
		},
		{ // A PTR record query for an existing ipv6 endpoint should return a record
			Qname: "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.d.c.b.a.4.3.2.1.ip6.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.PTR("1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.d.c.b.a.4.3.2.1.ip6.arpa. 5 IN PTR ep1b.svc1.testns.svc.cluster.local."),
			},
		},
		{ // A PTR record query for an existing ipv6 endpoint should return a record
			Qname: "a.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.7.7.0.0.0.0.d.f.ip6.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.PTR("a.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.7.7.0.0.0.0.d.f.ip6.arpa. 5 IN PTR ip6svc1ex.svc1.testns.svc.cluster.local."),
			},
		},
		{ // A PTR record query for an existing ipv6 endpoint should return a record
			Qname: "6.a.b.9.2.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.7.7.0.0.0.0.d.f.ip6.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.PTR("6.a.b.9.2.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.3.0.0.7.7.0.0.0.0.d.f.ip6.arpa. 5 IN PTR ip6svc1in.svc1.testns.svc.cluster.local."),
			},
		},
		{
			Qname: "101.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Ns: []dns.RR{
				test.SOA("0.10.in-addr.arpa.	300	IN	SOA	ns.dns.0.10.in-addr.arpa. hostmaster.0.10.in-addr.arpa. 1502782828 7200 1800 86400 60"),
			},
		},
		{
			Qname: "example.org.cluster.local.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Ns: []dns.RR{
				test.SOA("cluster.local.       300     IN      SOA     ns.dns.cluster.local. hostmaster.cluster.local. 1502989566 7200 1800 86400 60"),
			},
		},
	}

	ctx := context.TODO()
	for i, tc := range tests {
		r := tc.Msg()

		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d: expected no error, got %v", i, err)
			return
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d: got nil message and no error for: %s %d", i, r.Question[0].Name, r.Question[0].Qtype)
		}
		test.SortAndCheck(t, resp, tc)
	}
}
