// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

// etcd needs to be running on http://127.0.0.1:4001

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	backendetcd "github.com/skynetservices/skydns/backends/etcd"
	"github.com/skynetservices/skydns/msg"

	etcd "github.com/coreos/etcd/client"
	"github.com/miekg/dns"
)

func addService(t *testing.T, s *server, k string, ttl time.Duration, m *msg.Service) {
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := msg.PathWithWildcard(k)

	s.backend.(*backendetcd.Backend).Client().Set(ctx, path, string(b), &etcd.SetOptions{TTL: ttl})
}

func delService(t *testing.T, s *server, k string) {
	path, _ := msg.PathWithWildcard(k)
	backend.(*backendetcd.Backend).Client().Delete(ctx, path, &etcd.DeleteOptions{Recursive: false})
}

type rrSet []dns.RR

func (p rrSet) Len() int           { return len(p) }
func (p rrSet) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p rrSet) Less(i, j int) bool { return p[i].String() < p[j].String() }

func TestDNS(t *testing.T) {
	s := newTestServerDNSSEC(t, false)
	defer s.Stop()

	for _, serv := range services {
		addService(t, s, serv.Key, 0, serv)
		defer delService(t, s, serv.Key)
	}
	c := new(dns.Client)
	for _, tc := range dnsTestCases {
		m := new(dns.Msg)
		m.SetQuestion(tc.Qname, tc.Qtype)
		if tc.dnssec {
			m.SetEdns0(4096, true)
		}
		if tc.chaos {
			m.Question[0].Qclass = dns.ClassCHAOS
		}
		resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
		if err != nil {
			// try twice, be more resilent against remote lookups
			// timing out.
			resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
			if err != nil {
				t.Fatalf("failing: %s: %s\n", m.String(), err.Error())
			}
		}
		sort.Sort(rrSet(resp.Answer))
		sort.Sort(rrSet(resp.Ns))
		sort.Sort(rrSet(resp.Extra))
		fatal := false
		defer func() {
			if fatal {
				t.Logf("question: %s\n", m.Question[0].String())
				t.Logf("%s\n", resp)
			}
		}()
		if resp.Rcode != tc.Rcode {
			fatal = true
			t.Fatalf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
		}
		if len(resp.Answer) != len(tc.Answer) {
			fatal = true
			t.Fatalf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
		}
		for i, a := range resp.Answer {
			if a.Header().Name != tc.Answer[i].Header().Name {
				fatal = true
				t.Fatalf("answer %d should have a Header Name of %q, but has %q", i, tc.Answer[i].Header().Name, a.Header().Name)
			}
			if a.Header().Ttl != tc.Answer[i].Header().Ttl {
				fatal = true
				t.Fatalf("Answer %d should have a Header TTL of %d, but has %d", i, tc.Answer[i].Header().Ttl, a.Header().Ttl)
			}
			if a.Header().Rrtype != tc.Answer[i].Header().Rrtype {
				fatal = true
				t.Fatalf("answer %d should have a header response type of %d, but has %d", i, tc.Answer[i].Header().Rrtype, a.Header().Rrtype)
			}
			switch x := a.(type) {
			case *dns.SRV:
				if x.Priority != tc.Answer[i].(*dns.SRV).Priority {
					fatal = true
					t.Fatalf("answer %d should have a Priority of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Priority, x.Priority)
				}
				if x.Weight != tc.Answer[i].(*dns.SRV).Weight {
					fatal = true
					t.Fatalf("answer %d should have a Weight of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Weight, x.Weight)
				}
				if x.Port != tc.Answer[i].(*dns.SRV).Port {
					fatal = true
					t.Fatalf("answer %d should have a Port of %d, but has %d", i, tc.Answer[i].(*dns.SRV).Port, x.Port)
				}
				if x.Target != tc.Answer[i].(*dns.SRV).Target {
					fatal = true
					t.Fatalf("answer %d should have a Target of %q, but has %q", i, tc.Answer[i].(*dns.SRV).Target, x.Target)
				}
			case *dns.A:
				if x.A.String() != tc.Answer[i].(*dns.A).A.String() {
					fatal = true
					t.Fatalf("answer %d should have a Address of %q, but has %q", i, tc.Answer[i].(*dns.A).A.String(), x.A.String())
				}
			case *dns.AAAA:
				if x.AAAA.String() != tc.Answer[i].(*dns.AAAA).AAAA.String() {
					fatal = true
					t.Fatalf("answer %d should have a Address of %q, but has %q", i, tc.Answer[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				}
			case *dns.TXT:
				for j, txt := range x.Txt {
					if txt != tc.Answer[i].(*dns.TXT).Txt[j] {
						fatal = true
						t.Fatalf("answer %d should have a Txt of %q, but has %q", i, tc.Answer[i].(*dns.TXT).Txt[j], txt)
					}
				}
			case *dns.SOA:
				tt := tc.Answer[i].(*dns.SOA)
				if x.Ns != tt.Ns {
					fatal = true
					t.Fatalf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.PTR:
				tt := tc.Answer[i].(*dns.PTR)
				if x.Ptr != tt.Ptr {
					fatal = true
					t.Fatalf("PTR ptr should be %q, but is %q", x.Ptr, tt.Ptr)
				}
			case *dns.CNAME:
				tt := tc.Answer[i].(*dns.CNAME)
				if x.Target != tt.Target {
					fatal = true
					t.Fatalf("CNAME target should be %q, but is %q", x.Target, tt.Target)
				}
			case *dns.MX:
				tt := tc.Answer[i].(*dns.MX)
				if x.Mx != tt.Mx {
					t.Fatalf("MX Mx should be %q, but is %q", x.Mx, tt.Mx)
				}
				if x.Preference != tt.Preference {
					t.Fatalf("MX Preference should be %q, but is %q", x.Preference, tt.Preference)
				}
			}
		}
		if len(resp.Ns) != len(tc.Ns) {
			fatal = true
			t.Fatalf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
		}
		for i, n := range resp.Ns {
			switch x := n.(type) {
			case *dns.SOA:
				tt := tc.Ns[i].(*dns.SOA)
				if x.Ns != tt.Ns {
					fatal = true
					t.Fatalf("SOA nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			case *dns.NS:
				tt := tc.Ns[i].(*dns.NS)
				if x.Ns != tt.Ns {
					fatal = true
					t.Fatalf("NS nameserver should be %q, but is %q", x.Ns, tt.Ns)
				}
			}
		}
		if len(resp.Extra) != len(tc.Extra) {
			fatal = true
			t.Fatalf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
		}
		for i, e := range resp.Extra {
			switch x := e.(type) {
			case *dns.A:
				if x.A.String() != tc.Extra[i].(*dns.A).A.String() {
					fatal = true
					t.Fatalf("extra %d should have a address of %q, but has %q", i, tc.Extra[i].(*dns.A).A.String(), x.A.String())
				}
			case *dns.AAAA:
				if x.AAAA.String() != tc.Extra[i].(*dns.AAAA).AAAA.String() {
					fatal = true
					t.Fatalf("extra %d should have a address of %q, but has %q", i, tc.Extra[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				}
			case *dns.CNAME:
				tt := tc.Extra[i].(*dns.CNAME)
				if x.Target != tt.Target {
					// Super super gross hack.
					if x.Target == "a.ipaddr.skydns.test." && tt.Target == "b.ipaddr.skydns.test." {
						// These records are randomly choosen, either one is OK.
						continue
					}
					fatal = true
					t.Fatalf("CNAME target should be %q, but is %q", x.Target, tt.Target)
				}
			}
		}
	}
}

type dnsTestCase struct {
	Qname  string
	Qtype  uint16
	chaos  bool
	Rcode  int
	Answer []dns.RR
	Ns     []dns.RR
	Extra  []dns.RR
}

// Note the key is encoded as DNS name, while in "reality" it is a etcd path.
var services = []*msg.Service{
	{Host: "server1", Port: 8080, Key: "100.server1.development.region1.skydns.test."},
	{Host: "server2", Port: 80, Key: "101.server2.production.region1.skydns.test."},
	{Host: "server4", Port: 80, Priority: 333, Key: "102.server4.development.region6.skydns.test."},
}

var dnsTestCases = []dnsTestCase{
	// Full Name Test
	{
		Qname: "100.server1.development.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("100.server1.development.region1.skydns.test. 3600 SRV 10 100 8080 server1.")},
	},
	// A Record Test
	{
		Qname: "104.server1.development.region1.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// Multiple A Record Test
	{
		Qname: "ipaddr.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			newA("ipaddr.skydns.test. 3600 A 172.16.1.1"),
			newA("ipaddr.skydns.test. 3600 A 172.16.1.2"),
		},
	},
	// A Record Test with SRV
	{
		Qname: "104.server1.development.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("104.server1.development.region1.skydns.test. 3600 SRV 10 100 0 104.server1.development.region1.skydns.test.")},
		Extra:  []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// AAAAA Record Test
	{
		Qname: "105.server3.production.region2.skydns.test.", Qtype: dns.TypeAAAA,
		Answer: []dns.RR{newAAAA("105.server3.production.region2.skydns.test. 3600 AAAA 2001::8:8:8:8")},
	},
	// Multi SRV with the same target, should be dedupped.
	{
		Qname: "*.cname2.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("*.cname2.skydns.test. 3600 IN SRV 10 100 0 www.miek.nl."),
		},
		Extra: []dns.RR{
			newA("a.miek.nl. 3600 IN A 139.162.196.78"),
			newAAAA("a.miek.nl. 3600 IN AAAA 2a01:7e00::f03c:91ff:fef1:6735"),
			newCNAME("www.miek.nl. 3600 IN CNAME a.miek.nl."),
		},
	},
	// CNAME (unresolvable internal name)
	{
		Qname: "2.cname.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{},
		Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
	},
	// CNAME loop detection
	{
		Qname: "3.cname.skydns.test.", Qtype: dns.TypeA,
		Answer: []dns.RR{},
		Ns:     []dns.RR{newSOA("skydns.test. 60 SOA ns.dns.skydns.test. hostmaster.skydns.test. 1407441600 28800 7200 604800 60")},
	},
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
	// Priority Test
	{
		Qname: "region6.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{newSRV("region6.skydns.test. 3600 SRV 333 100 80 server4.")},
	},
	// Subdomain Test
	{
		Qname: "region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("region1.skydns.test. 3600 SRV 10 33 0 104.server1.development.region1.skydns.test."),
			newSRV("region1.skydns.test. 3600 SRV 10 33 80 server2"),
			newSRV("region1.skydns.test. 3600 SRV 10 33 8080 server1.")},
		Extra: []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// Subdomain Weight Test
	{
		Qname: "region5.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("region5.skydns.test. 3600 SRV 10 22 0 server2."),
			newSRV("region5.skydns.test. 3600 SRV 10 36 0 server1."),
			newSRV("region5.skydns.test. 3600 SRV 10 41 0 server3."),
			newSRV("region5.skydns.test. 3600 SRV 30 100 0 server4.")},
	},
	// Wildcard Test
	{
		Qname: "*.region1.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("*.region1.skydns.test. 3600 SRV 10 33 0 104.server1.development.region1.skydns.test."),
			newSRV("*.region1.skydns.test. 3600 SRV 10 33 80 server2"),
			newSRV("*.region1.skydns.test. 3600 SRV 10 33 8080 server1.")},
		Extra: []dns.RR{newA("104.server1.development.region1.skydns.test. 3600 A 10.0.0.1")},
	},
	// Wildcard Test
	{
		Qname: "production.*.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("production.*.skydns.test. 3600 IN SRV 10 50 0 105.server3.production.region2.skydns.test."),
			newSRV("production.*.skydns.test. 3600 IN SRV 10 50 80 server2.")},
		Extra: []dns.RR{newAAAA("105.server3.production.region2.skydns.test. 3600 IN AAAA 2001::8:8:8:8")},
	},
	// Wildcard Test
	{
		Qname: "production.any.skydns.test.", Qtype: dns.TypeSRV,
		Answer: []dns.RR{
			newSRV("production.any.skydns.test. 3600 IN SRV 10 50 0 105.server3.production.region2.skydns.test."),
			newSRV("production.any.skydns.test. 3600 IN SRV 10 50 80 server2.")},
		Extra: []dns.RR{newAAAA("105.server3.production.region2.skydns.test. 3600 IN AAAA 2001::8:8:8:8")},
	},
	// NXDOMAIN Test
	{
		Qname: "doesnotexist.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0"),
		},
	},
	// NODATA Test
	{
		Qname: "104.server1.development.region1.skydns.test.", Qtype: dns.TypeTXT,
		Ns: []dns.RR{newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
	},
	// NODATA Test 2
	{
		Qname: "100.server1.development.region1.skydns.test.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Ns:    []dns.RR{newSOA("skydns.test. 3600 SOA ns.dns.skydns.test. hostmaster.skydns.test. 0 0 0 0 0")},
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
}

func newA(rr string) *dns.A         { r, _ := dns.NewRR(rr); return r.(*dns.A) }
func newAAAA(rr string) *dns.AAAA   { r, _ := dns.NewRR(rr); return r.(*dns.AAAA) }
func newCNAME(rr string) *dns.CNAME { r, _ := dns.NewRR(rr); return r.(*dns.CNAME) }
func newSRV(rr string) *dns.SRV     { r, _ := dns.NewRR(rr); return r.(*dns.SRV) }
func newSOA(rr string) *dns.SOA     { r, _ := dns.NewRR(rr); return r.(*dns.SOA) }
func newNS(rr string) *dns.NS       { r, _ := dns.NewRR(rr); return r.(*dns.NS) }
func newPTR(rr string) *dns.PTR     { r, _ := dns.NewRR(rr); return r.(*dns.PTR) }
func newTXT(rr string) *dns.TXT     { r, _ := dns.NewRR(rr); return r.(*dns.TXT) }
func newMX(rr string) *dns.MX       { r, _ := dns.NewRR(rr); return r.(*dns.MX) }
