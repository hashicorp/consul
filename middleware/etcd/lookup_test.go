// +build net

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
	{Host: "server1", Port: 8080, Key: "a.server1.dev.region1.skydns.test."},
	{Host: "10.0.0.1", Port: 8080, Key: "a.server1.prod.region1.skydns.test."},
	{Host: "10.0.0.2", Port: 8080, Key: "b.server1.prod.region1.skydns.test."},
	{Host: "::1", Port: 8080, Key: "b.server6.prod.region1.skydns.test."},

	// CNAME dedup
	{Host: "www.miek.nl", Key: "a.miek.nl.skydns.test."},
	{Host: "www.miek.nl", Key: "b.miek.nl.skydns.test."},

	// Unresolvable internal name
	{Host: "unresolvable.skydns.test", Key: "cname.prod.region1.skydns.test."},
	// priority
	{Host: "server1", Priority: 333, Port: 8080, Key: "priority.skydns.test."},
	// Subdomain
	{Host: "server1", Port: 0, Key: "a.sub.region1.skydns.test."},
	{Host: "server2", Port: 80, Key: "b.sub.region1.skydns.test."},
	{Host: "10.0.0.1", Port: 8080, Key: "c.sub.region1.skydns.test."},
}

var dnsTestCases = []dnsTestCase{
	// SRV Test
	{
		Qname: "a.server1.dev.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("a.server1.dev.region1.skydns.test. 300 SRV 10 100 8080 server1.")},
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
		Answer: []dns.RR{newSRV("priority.skydns.test. 300 SRV 333 100 8080 server1.")},
	},
	// Subdomain Test
	{
		Qname: "sub.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("sub.region1.skydns.test. 300 IN SRV 10 33 0 server1."),
			newSRV("sub.region1.skydns.test. 300 IN SRV 10 33 80 server2."),
			newSRV("sub.region1.skydns.test. 300 IN SRV 10 33 8080 c.sub.region1.skydns.test."),
		},
		Extra: []dns.RR{newA("c.sub.region1.skydns.test. 300 IN A 10.0.0.1")},
	},
	// Multi SRV with the same target, should be dedupped.
	{
		Qname: "*.miek.nl.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("*.miek.nl.skydns.test. 300 IN SRV 10 100 0 www.miek.nl."),
		},
		// TODO(miek): bit stupid to rely on my home DNS setup for this...
		Extra: []dns.RR{
			// 303 ttl: don't care for the ttl on these RRs.
			newA("a.miek.nl. 303 IN A 139.162.196.78"),
			newAAAA("a.miek.nl. 303 IN AAAA 2a01:7e00::f03c:91ff:fef1:6735"),
			newCNAME("www.miek.nl. 303 IN CNAME a.miek.nl."),
		},
	},
	// CNAME (unresolvable internal name)
	{
		Qname: "cname.prod.region1.skydns.test.", Qtype: dns.TypeA,
		Ns: []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
	/*
		// CNAME (resolvable external name)
		{
			Qname: "external1.cname.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				newA("a.miek.nl. 60 IN A 139.162.196.78"),
				newCNAME("external1.cname.skydns.test. 60 IN CNAME www.miek.nl."),
				newCNAME("www.miek.nl. 60 IN CNAME a.miek.nl."),
			},
		},
		// CNAME (unresolvable external name)
		{
			Qname: "external2.cname.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{},
			Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
		},
		// CNAME loop detection
		{
			Qname: "3.cname.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{},
			Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
		},
		// Wildcard Test
		{
			Qname: "*.region1.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("*.region1.skydns.test. 300 SRV 10 33 0 104.server1.dev.region1.skydns.test."),
				newSRV("*.region1.skydns.test. 300 SRV 10 33 80 server2"),
				newSRV("*.region1.skydns.test. 300 SRV 10 33 8080 server1.")},
			Extra: []dns.RR{newA("104.server1.dev.region1.skydns.test. 300 A 10.0.0.1")},
		},
		// Wildcard Test
		{
			Qname: "prod.*.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("prod.*.skydns.test. 300 IN SRV 10 50 0 105.server3.prod.region2.skydns.test."),
				newSRV("prod.*.skydns.test. 300 IN SRV 10 50 80 server2.")},
			Extra: []dns.RR{newAAAA("105.server3.prod.region2.skydns.test. 300 IN AAAA 2001::8:8:8:8")},
		},
		// Wildcard Test
		{
			Qname: "prod.any.skydns.test.", Qtype: dns.TypeSRV,
			Answer: []dns.RR{
				newSRV("prod.any.skydns.test. 300 IN SRV 10 50 0 105.server3.prod.region2.skydns.test."),
				newSRV("prod.any.skydns.test. 300 IN SRV 10 50 80 server2.")},
			Extra: []dns.RR{newAAAA("105.server3.prod.region2.skydns.test. 300 IN AAAA 2001::8:8:8:8")},
		},
		// NODATA Test
		{
			Qname: "104.server1.dev.region1.skydns.test.", Qtype: dns.TypeTXT,
			Ns: []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
		},
		// NODATA Test 2
		{
			Qname: "100.server1.dev.region1.skydns.test.", Qtype: dns.TypeA,
			Rcode: dns.RcodeSuccess,
			Ns:    []dns.RR{newSOA("skydns.test. 300 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
		},
		{
			// One has group, the other has not...  Include the non-group always.
			Qname: "dom2.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				newA("dom2.skydns.test. IN A 127.0.0.1"),
				newA("dom2.skydns.test. IN A 127.0.0.2"),
			},
		},
		{
			// The groups differ.
			Qname: "dom1.skydns.test.", Qtype: dns.TypeA,
			Answer: []dns.RR{
				newA("dom1.skydns.test. IN A 127.0.0.1"),
			},
		},
	*/
}
