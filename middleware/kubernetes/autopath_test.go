package kubernetes

import (
	"net"
	"testing"

	"github.com/coredns/coredns/middleware/kubernetes/autopath"
	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

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

// Disabled because broken.
func testServeDNSAutoPath(t *testing.T) {

	k := Kubernetes{}
	k.Zones = []string{"cluster.local."}
	_, cidr, _ := net.ParseCIDR("10.0.0.0/8")

	k.ReverseCidrs = []net.IPNet{*cidr}
	k.Federations = []Federation{{name: "fed", zone: "federal.test."}}
	k.APIConn = &APIConnServeTest{}
	k.autoPath = new(autopath.AutoPath)
	k.autoPath.HostSearchPath = []string{"hostdom.test"}
	k.interfaceAddrsFunc = localPodIP
	k.Next = nextHandler(nextMap)

	ctx := context.TODO()
	runServeDNSTests(ctx, t, autopathCases, k)
	runServeDNSTests(ctx, t, autopathBareSearch, k)

	// Set ndots to 2 for the ndots test cases
	k.autoPath.NDots = 2
	runServeDNSTests(ctx, t, autopath2NDotsCases, k)
	k.autoPath.NDots = defautNdots
	// Disable the NXDOMAIN override (enabled by default)
	k.autoPath.OnNXDOMAIN = dns.RcodeNameError
	runServeDNSTests(ctx, t, autopathCases, k)
	runServeDNSTests(ctx, t, autopathBareSearchExpectNameErr, k)
}

var nextMap = map[dns.Question]dns.Msg{
	{Name: "test1.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("test1.hostdom.test.	0 IN	A	11.22.33.44")},
	},
	{Name: "test2.interwebs.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("test2.interwebs. 0	IN	A	55.66.77.88")},
	},
	{Name: "test2.interwebs.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.AAAA("test2.interwebs.  0 IN  AAAA  5555:6666:7777::8888")},
	},
	{Name: "foo.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("foo.hostdom.test. 0	IN	A	11.22.33.44")},
	},
	{Name: "foo.foo.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("foo.foo.hostdom.test. 0	IN	A	11.22.33.44")},
	},
	{Name: "foo.foo.foo.hostdom.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}: {
		Answer: []dns.RR{test.A("foo.foo.foo.hostdom.test. 0	IN	A	11.22.33.44")},
	},
}

// nextHandler returns a Handler that returns an answer for the question in the
// request per the question->answer map qMap.
func nextHandler(mm map[dns.Question]dns.Msg) test.Handler {
	return test.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetReply(r)

		msg, ok := mm[r.Question[0]]
		if !ok {
			r.Rcode = dns.RcodeNameError
			w.WriteMsg(m)
			return r.Rcode, nil
		}
		r.Rcode = dns.RcodeSuccess
		m.Answer = append(m.Answer, msg.Answer...)
		w.WriteMsg(m)
		return r.Rcode, nil
	})
}
