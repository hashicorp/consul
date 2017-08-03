package kubernetes

import (
	"net"
	"sort"
	"testing"

	"github.com/coredns/coredns/middleware/pkg/dnsrecorder"
	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"k8s.io/client-go/1.5/pkg/api"
)

var dnsTestCases = map[string](*test.Case){
	"A Service": {
		Qname: "svc1.testns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
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
	// TODO A External
	"CNAME External": {
		Qname: "external.testns.svc.cluster.local.", Qtype: dns.TypeCNAME,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("external.testns.svc.cluster.local.	0	IN	CNAME	ext.interwebs.test."),
		},
	},
	"A Service (Local Federated)": {
		Qname: "svc1.testns.fed.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.testns.fed.svc.cluster.local.	0	IN	A	10.0.0.1"),
		},
	},
	"PTR Service": {
		Qname: "1.0.0.10.in-addr.arpa.", Qtype: dns.TypePTR,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.PTR("1.0.0.10.in-addr.arpa.	0	IN	PTR	svc1.testns.svc.cluster.local."),
		},
	},
	// TODO A Service (Remote Federated)
	"CNAME Service (Remote Federated)": {
		Qname: "svc0.testns.fed.svc.cluster.local.", Qtype: dns.TypeCNAME,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("svc0.testns.fed.svc.cluster.local.	0	IN	CNAME	svc0.testns.fed.svc.fd-az.fd-r.federal.test."),
		},
	},
	"AAAA Service (existing service)": {
		Qname: "svc1.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode:  dns.RcodeSuccess,
		Answer: []dns.RR{},
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"AAAA Service (non-existing service)": {
		Qname: "svc0.testns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
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
}

var podModeDisabledCases = map[string](*test.Case){

	"A Record Pod mode = Case 1": {
		Qname: "10-240-0-1.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Error:  errPodsDisabled,
		Answer: []dns.RR{},
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},

	"A Record Pod mode = Case 2": {
		Qname: "172-0-0-2.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Error:  errPodsDisabled,
		Answer: []dns.RR{},
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
}

var podModeInsecureCases = map[string](*test.Case){

	"A Record Pod mode = Case 1": {
		Qname: "10-240-0-1.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("10-240-0-1.podns.pod.cluster.local.	0	IN	A	10.240.0.1"),
		},
	},

	"A Record Pod mode = Case 2": {
		Qname: "172-0-0-2.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("172-0-0-2.podns.pod.cluster.local.	0	IN	A	172.0.0.2"),
		},
	},
}

var podModeVerifiedCases = map[string](*test.Case){

	"A Record Pod mode = Case 1": {
		Qname: "10-240-0-1.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("10-240-0-1.podns.pod.cluster.local.	0	IN	A	10.240.0.1"),
		},
	},

	"A Record Pod mode = Case 2": {
		Qname: "172-0-0-2.podns.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
}

var autopathCases = map[string](*test.Case){
	"A Autopath Service (Second Search)": {
		Qname: "svc1.testns.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("svc1.testns.podns.svc.cluster.local.	0	IN	CNAME	svc1.testns.svc.cluster.local."),
			test.A("svc1.testns.svc.cluster.local.	0	IN	A	10.0.0.1"),
		},
	},
	"A Autopath Service (Third Search)": {
		Qname: "svc1.testns.svc.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("svc1.testns.svc.podns.svc.cluster.local.	0	IN	CNAME	svc1.testns.svc.cluster.local."),
			test.A("svc1.testns.svc.cluster.local.	0	IN	A	10.0.0.1"),
		},
	},
	"A Autopath Next Middleware (Host Domain Search)": {
		Qname: "test1.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("test1.podns.svc.cluster.local.	0	IN	CNAME	test1.hostdom.test."),
			test.A("test1.hostdom.test.	0	IN	A	11.22.33.44"),
		},
	},
	"A Autopath Service (Bare Search)": {
		Qname: "svc1.testns.svc.cluster.local.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("svc1.testns.svc.cluster.local.podns.svc.cluster.local.	0	IN	CNAME	svc1.testns.svc.cluster.local."),
			test.A("svc1.testns.svc.cluster.local.	0	IN	A	10.0.0.1"),
		},
	},
	"A Autopath Next Middleware (Bare Search)": {
		Qname: "test2.interwebs.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("test2.interwebs.podns.svc.cluster.local.	0	IN	CNAME	test2.interwebs."),
			test.A("test2.interwebs.	0	IN	A	55.66.77.88"),
		},
	},
	"AAAA Autopath Next Middleware (Bare Search)": {
		Qname: "test2.interwebs.podns.svc.cluster.local.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("test2.interwebs.podns.svc.cluster.local.	0	IN	CNAME	test2.interwebs."),
			test.AAAA("test2.interwebs.	0	IN	AAAA	5555:6666:7777::8888"),
		},
	},
}
var autopathBareSearch = map[string](*test.Case){
	"A Autopath Next Middleware (Bare Search) Non-existing OnNXDOMAIN default": {
		Qname: "nothere.interwebs.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeSuccess,
		Answer: []dns.RR{},
	},
}
var autopathBareSearchExpectNameErr = map[string](*test.Case){
	"A Autopath Next Middleware (Bare Search) Non-existing OnNXDOMAIN disabled": {
		Qname: "nothere.interwebs.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
	},
}
var autopath2NDotsCases = map[string](*test.Case){
	"A Service (0 Dots)": {
		Qname: "foo.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"A Service (1 Dots)": {
		Qname: "foo.foo.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeNameError,
		Answer: []dns.RR{},
		Ns: []dns.RR{
			test.SOA("cluster.local.	300	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1499347823 7200 1800 86400 60"),
		},
	},
	"A Service (2 Dots)": {
		Qname: "foo.foo.foo.podns.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("foo.foo.foo.hostdom.test.	0	IN	A	11.22.33.44"),
			test.CNAME("foo.foo.foo.podns.svc.cluster.local.	0	IN	CNAME	foo.foo.foo.hostdom.test."),
		},
	},
}

func TestServeDNS(t *testing.T) {

	k := Kubernetes{Zones: []string{"cluster.local."}}
	_, cidr, _ := net.ParseCIDR("10.0.0.0/8")

	k.ReverseCidrs = []net.IPNet{*cidr}
	k.Federations = []Federation{{name: "fed", zone: "federal.test."}}
	k.APIConn = &APIConnServeTest{}
	k.AutoPath.Enabled = true
	k.AutoPath.HostSearchPath = []string{"hostdom.test"}
	//k.Proxy = test.MockHandler(nextMWMap)
	k.Next = testHandler(nextMWMap)

	ctx := context.TODO()
	runServeDNSTests(t, dnsTestCases, k, ctx)
	runServeDNSTests(t, autopathCases, k, ctx)
	runServeDNSTests(t, autopathBareSearch, k, ctx)

	//Set PodMode to Disabled
	k.PodMode = PodModeDisabled
	runServeDNSTests(t, podModeDisabledCases, k, ctx)
	//Set PodMode to Insecure
	k.PodMode = PodModeInsecure
	runServeDNSTests(t, podModeInsecureCases, k, ctx)
	//Set PodMode to Verified
	k.PodMode = PodModeVerified
	runServeDNSTests(t, podModeVerifiedCases, k, ctx)

	// Set ndots to 2 for the ndots test cases
	k.AutoPath.NDots = 2
	runServeDNSTests(t, autopath2NDotsCases, k, ctx)
	k.AutoPath.NDots = defautNdots
	// Disable the NXDOMAIN override (enabled by default)
	k.OnNXDOMAIN = dns.RcodeNameError
	runServeDNSTests(t, autopathCases, k, ctx)
	runServeDNSTests(t, autopathBareSearchExpectNameErr, k, ctx)

}

func runServeDNSTests(t *testing.T, dnsTestCases map[string](*test.Case), k Kubernetes, ctx context.Context) {
	for testname, tc := range dnsTestCases {
		testname = "\nTest Case \"" + testname + "\""
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

		// Before sorting, make sure that CNAMES do not appear after their target records
		for i, c := range resp.Answer {
			if c.Header().Rrtype != dns.TypeCNAME {
				continue
			}
			for _, a := range resp.Answer[:i] {
				if a.Header().Name != c.(*dns.CNAME).Target {
					continue
				}
				t.Errorf("%v: CNAME found after target record\n", testname)
				t.Logf("%v Received:\n %v\n", testname, resp)

			}
		}

		sort.Sort(test.RRSet(resp.Answer))
		sort.Sort(test.RRSet(resp.Ns))
		sort.Sort(test.RRSet(resp.Extra))
		sort.Sort(test.RRSet(tc.Answer))
		sort.Sort(test.RRSet(tc.Ns))
		sort.Sort(test.RRSet(tc.Extra))

		if !test.Header(t, *tc, resp) {
			t.Logf("%v Received:\n %v\n", testname, resp)
			continue
		}
		if !test.Section(t, *tc, test.Answer, resp.Answer) {
			t.Logf("%v Received:\n %v\n", testname, resp)
		}
		if !test.Section(t, *tc, test.Ns, resp.Ns) {
			t.Logf("%v Received:\n %v\n", testname, resp)
		}
		if !test.Section(t, *tc, test.Extra, resp.Extra) {
			t.Logf("%v Received:\n %v\n", testname, resp)
		}
	}
}

// next middleware question->answer map

var nextMWMap = map[dns.Question]dns.Msg{
	{Name: "test1.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("test1.hostdom.test.	0	IN	A	11.22.33.44")},
	},
	{Name: "test2.interwebs.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("test2.interwebs.	0	IN	A	55.66.77.88")},
	},
	{Name: "test2.interwebs.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.AAAA("test2.interwebs.  0  IN  AAAA  5555:6666:7777::8888")},
	},
	{Name: "foo.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("foo.hostdom.test.	0	IN	A	11.22.33.44")},
	},
	{Name: "foo.foo.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("foo.foo.hostdom.test.	0	IN	A	11.22.33.44")},
	},
	{Name: "foo.foo.foo.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("foo.foo.foo.hostdom.test.	0	IN	A	11.22.33.44")},
	},
}

// testHandler returns a Handler that returns an answer for the question in the
// request per the question->answer map qMap.
func testHandler(qMap map[dns.Question]dns.Msg) test.Handler {
	return test.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetReply(r)
		msg, ok := qMap[r.Question[0]]
		if !ok {
			r.Rcode = dns.RcodeNameError
			return dns.RcodeNameError, nil
		}
		r.Rcode = dns.RcodeSuccess
		m.Answer = append(m.Answer, msg.Answer...)
		m.Extra = append(m.Extra, msg.Extra...)
		w.WriteMsg(m)
		return dns.RcodeSuccess, nil
	})
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
			Labels: map[string]string{
				labelRegion:           "fd-r",
				labelAvailabilityZone: "fd-az",
			},
		},
	}, nil
}
