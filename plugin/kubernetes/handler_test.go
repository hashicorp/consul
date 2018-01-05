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

var dnsTestCases = []test.Case{
	// A Service
	{
		Qname: "svc1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.testns.svc.cluster.local.	5	IN	A	10.0.0.1"),
		},
	},
	// A Service (wildcard)
	{
		Qname: "svc1.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.*.svc.cluster.local.  5       IN      A       10.0.0.1"),
		},
	},
	{
		Qname: "svc1.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("svc1.testns.svc.cluster.local.	5	IN	SRV	0 100 80 svc1.testns.svc.cluster.local.")},
		Extra: []dns.RR{test.A("svc1.testns.svc.cluster.local.  5       IN      A       10.0.0.1")},
	},
	// SRV Service (wildcard)
	{
		Qname: "svc1.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("svc1.*.svc.cluster.local.	5	IN	SRV	0 100 80 svc1.testns.svc.cluster.local.")},
		Extra: []dns.RR{test.A("svc1.testns.svc.cluster.local.  5       IN      A       10.0.0.1")},
	},
	// SRV Service (wildcards)
	{
		Qname: "*.any.svc1.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("*.any.svc1.*.svc.cluster.local.	5	IN	SRV	0 100 80 svc1.testns.svc.cluster.local.")},
		Extra: []dns.RR{test.A("svc1.testns.svc.cluster.local.  5       IN      A       10.0.0.1")},
	},
	// A Service (wildcards)
	{
		Qname: "*.any.svc1.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("*.any.svc1.*.svc.cluster.local.  5       IN      A       10.0.0.1"),
		},
	},
	// SRV Service Not udp/tcp
	{
		Qname: "*._not-udp-or-tcp.svc1.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	// SRV Service
	{
		Qname: "_http._tcp.svc1.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc1.testns.svc.cluster.local.	5	IN	SRV	0 100 80 svc1.testns.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc1.testns.svc.cluster.local.	5	IN	A	10.0.0.1"),
		},
	},
	// A Service (Headless)
	{
		Qname: "hdls1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("hdls1.testns.svc.cluster.local.	5	IN	A	172.0.0.2"),
			test.A("hdls1.testns.svc.cluster.local.	5	IN	A	172.0.0.3"),
		},
	},
	// A pod ip
	{
		Qname: "172-0-0-2.hdls1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("172-0-0-2.hdls1.testns.svc.cluster.local.	5	IN	A	172.0.0.2"),
		},
	},
	// A pod ip
	{
		Qname: "172-0-0-3.hdls1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("172-0-0-3.hdls1.testns.svc.cluster.local.	5	IN	A	172.0.0.3"),
		},
	},
	// SRV Service (Headless)
	{
		Qname: "_http._tcp.hdls1.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.hdls1.testns.svc.cluster.local.	5	IN	SRV	0 25 80 172-0-0-2.hdls1.testns.svc.cluster.local."),
			test.SRV("_http._tcp.hdls1.testns.svc.cluster.local.	5	IN	SRV	0 25 80 172-0-0-3.hdls1.testns.svc.cluster.local."),
			test.SRV("_http._tcp.hdls1.testns.svc.cluster.local.	5	IN	SRV	0 25 80 5678-abcd--1.hdls1.testns.svc.cluster.local."),
			test.SRV("_http._tcp.hdls1.testns.svc.cluster.local.	5	IN	SRV	0 25 80 5678-abcd--2.hdls1.testns.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("172-0-0-2.hdls1.testns.svc.cluster.local.	5	IN	A	172.0.0.2"),
			test.A("172-0-0-3.hdls1.testns.svc.cluster.local.	5	IN	A	172.0.0.3"),
			test.AAAA("5678-abcd--1.hdls1.testns.svc.cluster.local.	5	IN	AAAA	5678:abcd::1"),
			test.AAAA("5678-abcd--2.hdls1.testns.svc.cluster.local.	5	IN	AAAA	5678:abcd::2"),
		},
	},
	// AAAA
	{
		Qname: "5678-abcd--2.hdls1.testns.svc.cluster.local", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.AAAA("5678-abcd--2.hdls1.testns.svc.cluster.local.	5	IN	AAAA	5678:abcd::2")},
	},
	// CNAME External
	{
		Qname: "external.testns.svc.cluster.local.", Qtype: dns.TypeCNAME,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("external.testns.svc.cluster.local.	5	IN	CNAME	ext.interwebs.test."),
		},
	},
	// AAAA Service (with an existing A record, but no AAAA record)
	{
		Qname: "svc1.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	// AAAA Service (non-existing service)
	{
		Qname: "svc0.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	// A Service (non-existing service)
	{
		Qname: "svc0.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	// A Service (non-existing namespace)
	{
		Qname: "svc0.svc-nons.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	// TXT Schema
	{
		Qname: "dns-version.cluster.local.", Qtype: dns.TypeTXT,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.TXT("dns-version.cluster.local 28800 IN TXT 1.0.1"),
		},
	},
	// A Service (Headless) does not exist
	{
		Qname: "bogusendpoint.hdls1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	// A Service does not exist
	{
		Qname: "bogusendpoint.svc0.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	// AAAA Service
	{
		Qname: "svc6.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.AAAA("svc6.testns.svc.cluster.local.	5	IN	AAAA	1234:abcd::1"),
		},
	},
	// SRV
	{
		Qname: "_http._tcp.svc6.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc6.testns.svc.cluster.local.	5	IN	SRV	0 100 80 svc6.testns.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.AAAA("svc6.testns.svc.cluster.local.	5	IN	AAAA	1234:abcd::1"),
		},
	},
	// AAAA Service (Headless)
	{
		Qname: "hdls1.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.AAAA("hdls1.testns.svc.cluster.local.	5	IN	AAAA	5678:abcd::1"),
			test.AAAA("hdls1.testns.svc.cluster.local.	5	IN	AAAA	5678:abcd::2"),
		},
	},
	// AAAA Endpoint
	{
		Qname: "5678-abcd--1.hdls1.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.AAAA("5678-abcd--1.hdls1.testns.svc.cluster.local.	5	IN	AAAA	5678:abcd::1"),
		},
	},
}

func TestServeDNS(t *testing.T) {

	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	ctx := context.TODO()

	for i, tc := range dnsTestCases {
		r := tc.Msg()

		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d expected no error, got %v", i, err)
			return
		}
		if tc.Error != nil {
			continue
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d, got nil message and no error for %q", i, r.Question[0].Name)
		}

		// Before sorting, make sure that CNAMES do not appear after their target records
		test.CNAMEOrder(t, resp)

		test.SortAndCheck(t, resp, tc)
	}
}

type APIConnServeTest struct{}

func (APIConnServeTest) HasSynced() bool                        { return true }
func (APIConnServeTest) Run()                                   { return }
func (APIConnServeTest) Stop() error                            { return nil }
func (APIConnServeTest) EpIndexReverse(string) []*api.Endpoints { return nil }
func (APIConnServeTest) SvcIndexReverse(string) []*api.Service  { return nil }

func (APIConnServeTest) PodIndex(string) []*api.Pod {
	a := []*api.Pod{{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "podns",
		},
		Status: api.PodStatus{
			PodIP: "10.240.0.1", // Remote IP set in test.ResponseWriter
		},
	}}
	return a
}

var svcIndex = map[string][]*api.Service{
	"svc1.testns": {{
		ObjectMeta: meta.ObjectMeta{
			Name:      "svc1",
			Namespace: "testns",
		},
		Spec: api.ServiceSpec{
			ClusterIP: "10.0.0.1",
			Ports: []api.ServicePort{{
				Name:     "http",
				Protocol: "tcp",
				Port:     80,
			}},
		},
	}},
	"svc6.testns": {{
		ObjectMeta: meta.ObjectMeta{
			Name:      "svc6",
			Namespace: "testns",
		},
		Spec: api.ServiceSpec{
			ClusterIP: "1234:abcd::1",
			Ports: []api.ServicePort{{
				Name:     "http",
				Protocol: "tcp",
				Port:     80,
			}},
		},
	}},
	"hdls1.testns": {{
		ObjectMeta: meta.ObjectMeta{
			Name:      "hdls1",
			Namespace: "testns",
		},
		Spec: api.ServiceSpec{
			ClusterIP: api.ClusterIPNone,
		},
	}},
	"external.testns": {{
		ObjectMeta: meta.ObjectMeta{
			Name:      "external",
			Namespace: "testns",
		},
		Spec: api.ServiceSpec{
			ExternalName: "ext.interwebs.test",
			Ports: []api.ServicePort{{
				Name:     "http",
				Protocol: "tcp",
				Port:     80,
			}},
			Type: api.ServiceTypeExternalName,
		},
	}},
}

func (APIConnServeTest) SvcIndex(s string) []*api.Service {
	return svcIndex[s]
}

func (APIConnServeTest) ServiceList() []*api.Service {
	var svcs []*api.Service
	for _, svc := range svcIndex {
		svcs = append(svcs, svc...)
	}
	return svcs
}

var epsIndex = map[string][]*api.Endpoints{
	"svc1.testns": {{
		Subsets: []api.EndpointSubset{
			{
				Addresses: []api.EndpointAddress{
					{
						IP:       "172.0.0.1",
						Hostname: "ep1a",
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
	}},
	"hdls1.testns": {{
		Subsets: []api.EndpointSubset{
			{
				Addresses: []api.EndpointAddress{
					{
						IP: "172.0.0.2",
					},
					{
						IP: "172.0.0.3",
					},
					{
						IP: "5678:abcd::1",
					},
					{
						IP: "5678:abcd::2",
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
			Name:      "hdls1",
			Namespace: "testns",
		},
	}},
}

func (APIConnServeTest) EpIndex(s string) []*api.Endpoints {
	return epsIndex[s]
}

func (APIConnServeTest) EndpointsList() []*api.Endpoints {
	var eps []*api.Endpoints
	for _, ep := range epsIndex {
		eps = append(eps, ep...)
	}
	return eps

}

func (APIConnServeTest) GetNodeByName(name string) (*api.Node, error) {
	return &api.Node{
		ObjectMeta: meta.ObjectMeta{
			Name: "test.node.foo.bar",
		},
	}, nil
}

func (APIConnServeTest) GetNamespaceByName(name string) (*api.Namespace, error) {
	if name == "pod-nons" { // handler_pod_verified_test.go uses this for non-existent namespace.
		return &api.Namespace{}, nil
	}
	return &api.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
	}, nil
}
