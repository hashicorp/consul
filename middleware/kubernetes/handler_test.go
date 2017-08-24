package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/middleware/pkg/dnsrecorder"
	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"k8s.io/client-go/1.5/pkg/api"
)

var dnsTestCases = map[string](test.Case){
	"A Service": {
		Qname: "svc1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.testns.svc.cluster.local.	0	IN	A	10.0.0.1"),
		},
	},
	"A Service (wildcard)": {
		Qname: "svc1.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.*.svc.cluster.local.  0       IN      A       10.0.0.1"),
		},
	},
	"SRV Service (wildcard)": {
		Qname: "svc1.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("svc1.*.svc.cluster.local.	0	IN	SRV	0 100 80 svc1.testns.svc.cluster.local.")},
		Extra: []dns.RR{test.A("svc1.testns.svc.cluster.local.  0       IN      A       10.0.0.1")},
	},
	"SRV Service (wildcards)": {
		Qname: "*.any.svc1.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("*.any.svc1.*.svc.cluster.local.	0	IN	SRV	0 100 80 svc1.testns.svc.cluster.local.")},
		Extra: []dns.RR{test.A("svc1.testns.svc.cluster.local.  0       IN      A       10.0.0.1")},
	},
	"A Service (wildcards)": {
		Qname: "*.any.svc1.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("*.any.svc1.*.svc.cluster.local.  0       IN      A       10.0.0.1"),
		},
	},
	"SRV Service Not udp/tcp": {
		Qname: "*._not-udp-or-tcp.svc1.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"SRV Service": {
		Qname: "_http._tcp.svc1.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc1.testns.svc.cluster.local.	0	IN	SRV	0 100 80 svc1.testns.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc1.testns.svc.cluster.local.	0	IN	A	10.0.0.1"),
		},
	},
	"A Service (Headless)": {
		Qname: "hdls1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("hdls1.testns.svc.cluster.local.	0	IN	A	172.0.0.2"),
			test.A("hdls1.testns.svc.cluster.local.	0	IN	A	172.0.0.3"),
		},
	},
	"SRV Service (Headless)": {
		Qname: "_http._tcp.hdls1.testns.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.hdls1.testns.svc.cluster.local.	0	IN	SRV	0 50 80 172-0-0-2.hdls1.testns.svc.cluster.local."),
			test.SRV("_http._tcp.hdls1.testns.svc.cluster.local.	0	IN	SRV	0 50 80 172-0-0-3.hdls1.testns.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("172-0-0-2.hdls1.testns.svc.cluster.local.	0	IN	A	172.0.0.2"),
			test.A("172-0-0-3.hdls1.testns.svc.cluster.local.	0	IN	A	172.0.0.3"),
		},
	},
	"CNAME External": {
		Qname: "external.testns.svc.cluster.local.", Qtype: dns.TypeCNAME,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("external.testns.svc.cluster.local.	0	IN	CNAME	ext.interwebs.test."),
		},
	},
	"AAAA Service (existing service)": {
		Qname: "svc1.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"AAAA Service (non-existing service)": {
		Qname: "svc0.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"A Service (non-existing service)": {
		Qname: "svc0.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"TXT Schema": {
		Qname: "dns-version.cluster.local.", Qtype: dns.TypeTXT,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.TXT("dns-version.cluster.local 28800 IN TXT 1.0.1"),
		},
	},
	"A Service (Headless) does not exist": {
		Qname: "bogusendpoint.hdls1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"A Service does not exist": {
		Qname: "bogusendpoint.svc0.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
}

func TestServeDNS(t *testing.T) {

	k := New([]string{"cluster.local."})
	k.APIConn = &APIConnServeTest{}
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	ctx := context.TODO()

	for testname, tc := range dnsTestCases {
		r := tc.Msg()

		w := dnsrecorder.New(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("%v expected no error, got %v\n", testname, err)
			return
		}
		if tc.Error != nil {
			continue
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("got nil message and no error for %q: %s %d", testname, r.Question[0].Name, r.Question[0].Qtype)
		}

		// Before sorting, make sure that CNAMES do not appear after their target records
		test.CNAMEOrder(t, resp)

		test.SortAndCheck(t, resp, tc)
	}
}

type APIConnServeTest struct{}

func (APIConnServeTest) Run()        { return }
func (APIConnServeTest) Stop() error { return nil }

func (APIConnServeTest) PodIndex(string) []interface{} {
	a := make([]interface{}, 1)
	a[0] = &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Namespace: "podns",
		},
		Status: api.PodStatus{
			PodIP: "10.240.0.1", // Remote IP set in test.ResponseWriter
		},
	}
	return a
}

func (APIConnServeTest) ServiceList() []*api.Service {
	svcs := []*api.Service{
		{
			ObjectMeta: api.ObjectMeta{
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
		},
		{
			ObjectMeta: api.ObjectMeta{
				Name:      "hdls1",
				Namespace: "testns",
			},
			Spec: api.ServiceSpec{
				ClusterIP: api.ClusterIPNone,
			},
		},
		{
			ObjectMeta: api.ObjectMeta{
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
			},
		},
	}
	return svcs

}

func (APIConnServeTest) EndpointsList() api.EndpointsList {
	n := "test.node.foo.bar"

	return api.EndpointsList{
		Items: []api.Endpoints{
			{
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
				ObjectMeta: api.ObjectMeta{
					Name:      "svc1",
					Namespace: "testns",
				},
			},
			{
				Subsets: []api.EndpointSubset{
					{
						Addresses: []api.EndpointAddress{
							{
								IP: "172.0.0.2",
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
				ObjectMeta: api.ObjectMeta{
					Name:      "hdls1",
					Namespace: "testns",
				},
			},
			{
				Subsets: []api.EndpointSubset{
					{
						Addresses: []api.EndpointAddress{
							{
								IP: "172.0.0.3",
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
				ObjectMeta: api.ObjectMeta{
					Name:      "hdls1",
					Namespace: "testns",
				},
			},
			{
				Subsets: []api.EndpointSubset{
					{
						Addresses: []api.EndpointAddress{
							{
								IP:       "10.9.8.7",
								NodeName: &n,
							},
						},
					},
				},
			},
		},
	}
}

func (APIConnServeTest) GetNodeByName(name string) (api.Node, error) {
	return api.Node{
		ObjectMeta: api.ObjectMeta{
			Name: "test.node.foo.bar",
		},
	}, nil
}
