package test

import (
	"sort"
	"testing"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type sect int

const (
	// Answer is the answer section in an Msg.
	Answer sect = iota
	// Ns is the authoritative section in an Msg.
	Ns
	// Extra is the additional section in an Msg.
	Extra
)

// RRSet represents a list of RRs.
type RRSet []dns.RR

func (p RRSet) Len() int           { return len(p) }
func (p RRSet) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p RRSet) Less(i, j int) bool { return p[i].String() < p[j].String() }

// Case represents a test case that encapsulates various data from a query and response.
// Note that is the TTL of a record is 303 we don't compare it with the TTL.
type Case struct {
	Qname  string
	Qtype  uint16
	Rcode  int
	Do     bool
	Answer []dns.RR
	Ns     []dns.RR
	Extra  []dns.RR
	Error  error
}

// Msg returns a *dns.Msg embedded in c.
func (c Case) Msg() *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(c.Qname), c.Qtype)
	if c.Do {
		o := new(dns.OPT)
		o.Hdr.Name = "."
		o.Hdr.Rrtype = dns.TypeOPT
		o.SetDo()
		o.SetUDPSize(4096)
		m.Extra = []dns.RR{o}
	}
	return m
}

// A returns an A record from rr. It panics on errors.
func A(rr string) *dns.A { r, _ := dns.NewRR(rr); return r.(*dns.A) }

// AAAA returns an AAAA record from rr. It panics on errors.
func AAAA(rr string) *dns.AAAA { r, _ := dns.NewRR(rr); return r.(*dns.AAAA) }

// CNAME returns a CNAME record from rr. It panics on errors.
func CNAME(rr string) *dns.CNAME { r, _ := dns.NewRR(rr); return r.(*dns.CNAME) }

// DNAME returns a DNAME record from rr. It panics on errors.
func DNAME(rr string) *dns.DNAME { r, _ := dns.NewRR(rr); return r.(*dns.DNAME) }

// SRV returns a SRV record from rr. It panics on errors.
func SRV(rr string) *dns.SRV { r, _ := dns.NewRR(rr); return r.(*dns.SRV) }

// SOA returns a SOA record from rr. It panics on errors.
func SOA(rr string) *dns.SOA { r, _ := dns.NewRR(rr); return r.(*dns.SOA) }

// NS returns an NS record from rr. It panics on errors.
func NS(rr string) *dns.NS { r, _ := dns.NewRR(rr); return r.(*dns.NS) }

// PTR returns a PTR record from rr. It panics on errors.
func PTR(rr string) *dns.PTR { r, _ := dns.NewRR(rr); return r.(*dns.PTR) }

// TXT returns a TXT record from rr. It panics on errors.
func TXT(rr string) *dns.TXT { r, _ := dns.NewRR(rr); return r.(*dns.TXT) }

// MX returns an MX record from rr. It panics on errors.
func MX(rr string) *dns.MX { r, _ := dns.NewRR(rr); return r.(*dns.MX) }

// RRSIG returns an RRSIG record from rr. It panics on errors.
func RRSIG(rr string) *dns.RRSIG { r, _ := dns.NewRR(rr); return r.(*dns.RRSIG) }

// NSEC returns an NSEC record from rr. It panics on errors.
func NSEC(rr string) *dns.NSEC { r, _ := dns.NewRR(rr); return r.(*dns.NSEC) }

// DNSKEY returns a DNSKEY record from rr. It panics on errors.
func DNSKEY(rr string) *dns.DNSKEY { r, _ := dns.NewRR(rr); return r.(*dns.DNSKEY) }

// DS returns a DS record from rr. It panics on errors.
func DS(rr string) *dns.DS { r, _ := dns.NewRR(rr); return r.(*dns.DS) }

// OPT returns an OPT record with UDP buffer size set to bufsize and the DO bit set to do.
func OPT(bufsize int, do bool) *dns.OPT {
	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	o.SetVersion(0)
	o.SetUDPSize(uint16(bufsize))
	if do {
		o.SetDo()
	}
	return o
}

// Header test if the header in resp matches the header as defined in tc.
func Header(t *testing.T, tc Case, resp *dns.Msg) bool {
	if resp.Rcode != tc.Rcode {
		t.Errorf("rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
		return false
	}

	if len(resp.Answer) != len(tc.Answer) {
		t.Errorf("answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
		return false
	}
	if len(resp.Ns) != len(tc.Ns) {
		t.Errorf("authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
		return false
	}
	if len(resp.Extra) != len(tc.Extra) {
		t.Errorf("additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
		return false
	}
	return true
}

// Section tests if the the section in tc matches rr.
func Section(t *testing.T, tc Case, sec sect, rr []dns.RR) bool {
	section := []dns.RR{}
	switch sec {
	case 0:
		section = tc.Answer
	case 1:
		section = tc.Ns
	case 2:
		section = tc.Extra
	}

	for i, a := range rr {
		if a.Header().Name != section[i].Header().Name {
			t.Errorf("rr %d should have a Header Name of %q, but has %q", i, section[i].Header().Name, a.Header().Name)
			return false
		}
		// 303 signals: don't care what the ttl is.
		if section[i].Header().Ttl != 303 && a.Header().Ttl != section[i].Header().Ttl {
			if _, ok := section[i].(*dns.OPT); !ok {
				// we check edns0 bufize on this one
				t.Errorf("rr %d should have a Header TTL of %d, but has %d", i, section[i].Header().Ttl, a.Header().Ttl)
				return false
			}
		}
		if a.Header().Rrtype != section[i].Header().Rrtype {
			t.Errorf("rr %d should have a header rr type of %d, but has %d", i, section[i].Header().Rrtype, a.Header().Rrtype)
			return false
		}

		switch x := a.(type) {
		case *dns.SRV:
			if x.Priority != section[i].(*dns.SRV).Priority {
				t.Errorf("rr %d should have a Priority of %d, but has %d", i, section[i].(*dns.SRV).Priority, x.Priority)
				return false
			}
			if x.Weight != section[i].(*dns.SRV).Weight {
				t.Errorf("rr %d should have a Weight of %d, but has %d", i, section[i].(*dns.SRV).Weight, x.Weight)
				return false
			}
			if x.Port != section[i].(*dns.SRV).Port {
				t.Errorf("rr %d should have a Port of %d, but has %d", i, section[i].(*dns.SRV).Port, x.Port)
				return false
			}
			if x.Target != section[i].(*dns.SRV).Target {
				t.Errorf("rr %d should have a Target of %q, but has %q", i, section[i].(*dns.SRV).Target, x.Target)
				return false
			}
		case *dns.RRSIG:
			if x.TypeCovered != section[i].(*dns.RRSIG).TypeCovered {
				t.Errorf("rr %d should have a TypeCovered of %d, but has %d", i, section[i].(*dns.RRSIG).TypeCovered, x.TypeCovered)
				return false
			}
			if x.Labels != section[i].(*dns.RRSIG).Labels {
				t.Errorf("rr %d should have a Labels of %d, but has %d", i, section[i].(*dns.RRSIG).Labels, x.Labels)
				return false
			}
			if x.SignerName != section[i].(*dns.RRSIG).SignerName {
				t.Errorf("rr %d should have a SignerName of %s, but has %s", i, section[i].(*dns.RRSIG).SignerName, x.SignerName)
				return false
			}
		case *dns.NSEC:
			if x.NextDomain != section[i].(*dns.NSEC).NextDomain {
				t.Errorf("rr %d should have a NextDomain of %s, but has %s", i, section[i].(*dns.NSEC).NextDomain, x.NextDomain)
				return false
			}
			// TypeBitMap
		case *dns.A:
			if x.A.String() != section[i].(*dns.A).A.String() {
				t.Errorf("rr %d should have a Address of %q, but has %q", i, section[i].(*dns.A).A.String(), x.A.String())
				return false
			}
		case *dns.AAAA:
			if x.AAAA.String() != section[i].(*dns.AAAA).AAAA.String() {
				t.Errorf("rr %d should have a Address of %q, but has %q", i, section[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
				return false
			}
		case *dns.TXT:
			for j, txt := range x.Txt {
				if txt != section[i].(*dns.TXT).Txt[j] {
					t.Errorf("rr %d should have a Txt of %q, but has %q", i, section[i].(*dns.TXT).Txt[j], txt)
					return false
				}
			}
		case *dns.SOA:
			tt := section[i].(*dns.SOA)
			if x.Ns != tt.Ns {
				t.Errorf("SOA nameserver should be %q, but is %q", tt.Ns, x.Ns)
				return false
			}
		case *dns.PTR:
			tt := section[i].(*dns.PTR)
			if x.Ptr != tt.Ptr {
				t.Errorf("PTR ptr should be %q, but is %q", tt.Ptr, x.Ptr)
				return false
			}
		case *dns.CNAME:
			tt := section[i].(*dns.CNAME)
			if x.Target != tt.Target {
				t.Errorf("CNAME target should be %q, but is %q", tt.Target, x.Target)
				return false
			}
		case *dns.MX:
			tt := section[i].(*dns.MX)
			if x.Mx != tt.Mx {
				t.Errorf("MX Mx should be %q, but is %q", tt.Mx, x.Mx)
				return false
			}
			if x.Preference != tt.Preference {
				t.Errorf("MX Preference should be %q, but is %q", tt.Preference, x.Preference)
				return false
			}
		case *dns.NS:
			tt := section[i].(*dns.NS)
			if x.Ns != tt.Ns {
				t.Errorf("NS nameserver should be %q, but is %q", tt.Ns, x.Ns)
				return false
			}
		case *dns.OPT:
			tt := section[i].(*dns.OPT)
			if x.UDPSize() != tt.UDPSize() {
				t.Errorf("OPT UDPSize should be %d, but is %d", tt.UDPSize(), x.UDPSize())
				return false
			}
			if x.Do() != tt.Do() {
				t.Errorf("OPT DO should be %t, but is %t", tt.Do(), x.Do())
				return false
			}
		}
	}
	return true
}

// CNAMEOrder makes sure that CNAMES do not appear after their target records
func CNAMEOrder(t *testing.T, res *dns.Msg) {
	for i, c := range res.Answer {
		if c.Header().Rrtype != dns.TypeCNAME {
			continue
		}
		for _, a := range res.Answer[:i] {
			if a.Header().Name != c.(*dns.CNAME).Target {
				continue
			}
			t.Errorf("CNAME found after target record\n")
			t.Logf("%v\n", res)

		}
	}
}

// SortAndCheck sorts resp and the checks the header and three sections against the testcase in tc.
func SortAndCheck(t *testing.T, resp *dns.Msg, tc Case) {
	sort.Sort(RRSet(resp.Answer))
	sort.Sort(RRSet(resp.Ns))
	sort.Sort(RRSet(resp.Extra))

	if !Header(t, tc, resp) {
		t.Logf("%v\n", resp)
		return
	}

	if !Section(t, tc, Answer, resp.Answer) {
		t.Logf("%v\n", resp)
		return
	}
	if !Section(t, tc, Ns, resp.Ns) {
		t.Logf("%v\n", resp)
		return

	}
	if !Section(t, tc, Extra, resp.Extra) {
		t.Logf("%v\n", resp)
		return
	}
	return
}

// ErrorHandler returns a Handler that returns ServerFailure error when called.
func ErrorHandler() Handler {
	return HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return dns.RcodeServerFailure, nil
	})
}

// NextHandler returns a Handler that returns rcode and err.
func NextHandler(rcode int, err error) Handler {
	return HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		return rcode, err
	})
}

// Copied here to prevent an import cycle, so that we can define to above handlers.

type (
	// HandlerFunc is a convenience type like dns.HandlerFunc, except
	// ServeDNS returns an rcode and an error.
	HandlerFunc func(context.Context, dns.ResponseWriter, *dns.Msg) (int, error)

	// Handler interface defines a plugin.
	Handler interface {
		ServeDNS(context.Context, dns.ResponseWriter, *dns.Msg) (int, error)
		Name() string
	}
)

// ServeDNS implements the Handler interface.
func (f HandlerFunc) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return f(ctx, w, r)
}

// Name implements the Handler interface.
func (f HandlerFunc) Name() string { return "handlerfunc" }
