// +build etcd

package etcd

// etcd needs to be running on http://127.0.0.1:2379
// *and* needs connectivity to the internet for remotely resolving
// names.

import (
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/dns"
)

// Note the key is encoded as DNS name, while in "reality" it is a etcd path.
var services = []*msg.Service{
	{Host: "dev.server1", Port: 8080, Key: "a.server1.dev.region1.skydns.test."},
	{Host: "10.0.0.1", Port: 8080, Key: "a.server1.prod.region1.skydns.test."},
	{Host: "10.0.0.2", Port: 8080, Key: "b.server1.prod.region1.skydns.test."},
	{Host: "::1", Port: 8080, Key: "b.server6.prod.region1.skydns.test."},
	// Unresolvable internal name
	{Host: "unresolvable.skydns.test", Key: "cname.prod.region1.skydns.test."},
	// priority
	{Host: "priority.server1", Priority: 333, Port: 8080, Key: "priority.skydns.test."},
	// Subdomain
	{Host: "sub.server1", Port: 0, Key: "a.sub.region1.skydns.test."},
	{Host: "sub.server2", Port: 80, Key: "b.sub.region1.skydns.test."},
	{Host: "10.0.0.1", Port: 8080, Key: "c.sub.region1.skydns.test."},
	// Cname loop
	{Host: "a.cname.skydns.test", Key: "b.cname.skydns.test."},
	{Host: "b.cname.skydns.test", Key: "a.cname.skydns.test."},
}

var dnsTestCases = []dnsTestCase{
	// SRV Test
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.server1.dev.region1.skydns.test. 300 SRV 10 100 8080 dev.server1.")},
	},
	// SRV Test (case test)
	{
		Qname: "a.SERVer1.dEv.region1.skydns.tEst.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.SERVer1.dEv.region1.skydns.tEst. 300 SRV 10 100 8080 dev.server1.")},
	},
	// NXDOMAIN Test
	{
		Qname: "doesnotexist.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
	},
	// A Test
	{
		Qname: "a.server1.prod.region1.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("a.server1.prod.region1.skydns.test. 300 A 10.0.0.1")},
	},
	// SRV Test where target is IP address
	{
		Qname: "a.server1.prod.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.server1.prod.region1.skydns.test. 300 SRV 10 100 8080 a.server1.prod.region1.skydns.test.")},
		Extra:  []dns.RR{newA("a.server1.prod.region1.skydns.test. 300 A 10.0.0.1")},
	},
	// AAAA Test
	{
		Qname: "b.server6.prod.region1.skydns.test.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{newAAAA("b.server6.prod.region1.skydns.test. 300 AAAA ::1")},
	},
	// Multiple A Record Test
	{
		Qname: "server1.prod.region1.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newA("server1.prod.region1.skydns.test. 300 A 10.0.0.1"),
			newA("server1.prod.region1.skydns.test. 300 A 10.0.0.2"),
		},
	},
	// Priority Test
	{
		Qname: "priority.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("priority.skydns.test. 300 SRV 333 100 8080 priority.server1.")},
	},
	// Subdomain Test
	{
		Qname: "sub.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("sub.region1.skydns.test. 300 IN SRV 10 33 0 sub.server1."),
			newSRV("sub.region1.skydns.test. 300 IN SRV 10 33 80 sub.server2."),
			newSRV("sub.region1.skydns.test. 300 IN SRV 10 33 8080 c.sub.region1.skydns.test."),
		},
		Extra: []dns.RR{newA("c.sub.region1.skydns.test. 300 IN A 10.0.0.1")},
	},
	// CNAME (unresolvable internal name)
	{
		Qname: "cname.prod.region1.skydns.test.", Qtype: dns.TypeA,
		Ns: []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
	// Wildcard Test
	{
		Qname: "*.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("*.region1.skydns.test.	300	IN	SRV	10 14 0 sub.server1."),
			newSRV("*.region1.skydns.test.	300	IN	SRV	10 14 0 unresolvable.skydns.test."),
			newSRV("*.region1.skydns.test.	300	IN	SRV	10 14 80 sub.server2."),
			newSRV("*.region1.skydns.test.	300	IN	SRV	10 14 8080 a.server1.prod.region1.skydns.test."),
			newSRV("*.region1.skydns.test.	300	IN	SRV	10 14 8080 b.server1.prod.region1.skydns.test."),
			newSRV("*.region1.skydns.test.	300	IN	SRV	10 14 8080 b.server6.prod.region1.skydns.test."),
			newSRV("*.region1.skydns.test.	300	IN	SRV	10 14 8080 dev.server1."),
		},
		Extra: []dns.RR{
			newA("a.server1.prod.region1.skydns.test.	300	IN	A	10.0.0.1"),
			newA("b.server1.prod.region1.skydns.test.	300	IN	A	10.0.0.2"),
			newAAAA("b.server6.prod.region1.skydns.test.	300	IN	AAAA	::1"),
		},
	},
	// Wildcard Test
	{
		Qname: "prod.*.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{

			newSRV("prod.*.skydns.test.	300	IN	SRV	10 25 0 unresolvable.skydns.test."),
			newSRV("prod.*.skydns.test.	300	IN	SRV	10 25 8080 a.server1.prod.region1.skydns.test."),
			newSRV("prod.*.skydns.test.	300	IN	SRV	10 25 8080 b.server1.prod.region1.skydns.test."),
			newSRV("prod.*.skydns.test.	300	IN	SRV	10 25 8080 b.server6.prod.region1.skydns.test."),
		},
		Extra: []dns.RR{
			newA("a.server1.prod.region1.skydns.test.	300	IN	A	10.0.0.1"),
			newA("b.server1.prod.region1.skydns.test.	300	IN	A	10.0.0.2"),
			newAAAA("b.server6.prod.region1.skydns.test.	300	IN	AAAA	::1"),
		},
	},
	// Wildcard Test
	{
		Qname: "prod.any.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("prod.any.skydns.test.	300	IN	SRV	10 25 0 unresolvable.skydns.test."),
			newSRV("prod.any.skydns.test.	300	IN	SRV	10 25 8080 a.server1.prod.region1.skydns.test."),
			newSRV("prod.any.skydns.test.	300	IN	SRV	10 25 8080 b.server1.prod.region1.skydns.test."),
			newSRV("prod.any.skydns.test.	300	IN	SRV	10 25 8080 b.server6.prod.region1.skydns.test."),
		},
		Extra: []dns.RR{
			newA("a.server1.prod.region1.skydns.test.	300	IN	A	10.0.0.1"),
			newA("b.server1.prod.region1.skydns.test.	300	IN	A	10.0.0.2"),
			newAAAA("b.server6.prod.region1.skydns.test.	300	IN	AAAA	::1"),
		},
	},
	// CNAME loop detection
	{
		Qname: "a.cname.skydns.test.", Qtype: dns.TypeA,
		Ns: []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
	},
	// NODATA Test
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeTXT,
		Ns: []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
}
