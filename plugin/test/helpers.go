package test

import (
	"context"
	"fmt"
	"sort"

	"github.com/miekg/dns"
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

// HINFO returns a HINFO record from rr. It panics on errors.
func HINFO(rr string) *dns.HINFO { r, _ := dns.NewRR(rr); return r.(*dns.HINFO) }

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
func Header(tc Case, resp *dns.Msg) error {
	if resp.Rcode != tc.Rcode {
		return fmt.Errorf("Rcode is %q, expected %q", dns.RcodeToString[resp.Rcode], dns.RcodeToString[tc.Rcode])
	}

	if len(resp.Answer) != len(tc.Answer) {
		return fmt.Errorf("Answer for %q contained %d results, %d expected", tc.Qname, len(resp.Answer), len(tc.Answer))
	}
	if len(resp.Ns) != len(tc.Ns) {
		return fmt.Errorf("Authority for %q contained %d results, %d expected", tc.Qname, len(resp.Ns), len(tc.Ns))
	}
	if len(resp.Extra) != len(tc.Extra) {
		return fmt.Errorf("Additional for %q contained %d results, %d expected", tc.Qname, len(resp.Extra), len(tc.Extra))
	}
	return nil
}

// Section tests if the the section in tc matches rr.
func Section(tc Case, sec sect, rr []dns.RR) error {
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
			return fmt.Errorf("RR %d should have a Header Name of %q, but has %q", i, section[i].Header().Name, a.Header().Name)
		}
		// 303 signals: don't care what the ttl is.
		if section[i].Header().Ttl != 303 && a.Header().Ttl != section[i].Header().Ttl {
			if _, ok := section[i].(*dns.OPT); !ok {
				// we check edns0 bufize on this one
				return fmt.Errorf("RR %d should have a Header TTL of %d, but has %d", i, section[i].Header().Ttl, a.Header().Ttl)
			}
		}
		if a.Header().Rrtype != section[i].Header().Rrtype {
			return fmt.Errorf("RR %d should have a header rr type of %d, but has %d", i, section[i].Header().Rrtype, a.Header().Rrtype)
		}

		switch x := a.(type) {
		case *dns.SRV:
			if x.Priority != section[i].(*dns.SRV).Priority {
				return fmt.Errorf("RR %d should have a Priority of %d, but has %d", i, section[i].(*dns.SRV).Priority, x.Priority)
			}
			if x.Weight != section[i].(*dns.SRV).Weight {
				return fmt.Errorf("RR %d should have a Weight of %d, but has %d", i, section[i].(*dns.SRV).Weight, x.Weight)
			}
			if x.Port != section[i].(*dns.SRV).Port {
				return fmt.Errorf("RR %d should have a Port of %d, but has %d", i, section[i].(*dns.SRV).Port, x.Port)
			}
			if x.Target != section[i].(*dns.SRV).Target {
				return fmt.Errorf("RR %d should have a Target of %q, but has %q", i, section[i].(*dns.SRV).Target, x.Target)
			}
		case *dns.RRSIG:
			if x.TypeCovered != section[i].(*dns.RRSIG).TypeCovered {
				return fmt.Errorf("RR %d should have a TypeCovered of %d, but has %d", i, section[i].(*dns.RRSIG).TypeCovered, x.TypeCovered)
			}
			if x.Labels != section[i].(*dns.RRSIG).Labels {
				return fmt.Errorf("RR %d should have a Labels of %d, but has %d", i, section[i].(*dns.RRSIG).Labels, x.Labels)
			}
			if x.SignerName != section[i].(*dns.RRSIG).SignerName {
				return fmt.Errorf("RR %d should have a SignerName of %s, but has %s", i, section[i].(*dns.RRSIG).SignerName, x.SignerName)
			}
		case *dns.NSEC:
			if x.NextDomain != section[i].(*dns.NSEC).NextDomain {
				return fmt.Errorf("RR %d should have a NextDomain of %s, but has %s", i, section[i].(*dns.NSEC).NextDomain, x.NextDomain)
			}
			// TypeBitMap
		case *dns.A:
			if x.A.String() != section[i].(*dns.A).A.String() {
				return fmt.Errorf("RR %d should have a Address of %q, but has %q", i, section[i].(*dns.A).A.String(), x.A.String())
			}
		case *dns.AAAA:
			if x.AAAA.String() != section[i].(*dns.AAAA).AAAA.String() {
				return fmt.Errorf("RR %d should have a Address of %q, but has %q", i, section[i].(*dns.AAAA).AAAA.String(), x.AAAA.String())
			}
		case *dns.TXT:
			for j, txt := range x.Txt {
				if txt != section[i].(*dns.TXT).Txt[j] {
					return fmt.Errorf("RR %d should have a Txt of %q, but has %q", i, section[i].(*dns.TXT).Txt[j], txt)
				}
			}
		case *dns.HINFO:
			if x.Cpu != section[i].(*dns.HINFO).Cpu {
				return fmt.Errorf("RR %d should have a Cpu of %s, but has %s", i, section[i].(*dns.HINFO).Cpu, x.Cpu)
			}
			if x.Os != section[i].(*dns.HINFO).Os {
				return fmt.Errorf("RR %d should have a Os of %s, but has %s", i, section[i].(*dns.HINFO).Os, x.Os)
			}
		case *dns.SOA:
			tt := section[i].(*dns.SOA)
			if x.Ns != tt.Ns {
				return fmt.Errorf("SOA nameserver should be %q, but is %q", tt.Ns, x.Ns)
			}
		case *dns.PTR:
			tt := section[i].(*dns.PTR)
			if x.Ptr != tt.Ptr {
				return fmt.Errorf("PTR ptr should be %q, but is %q", tt.Ptr, x.Ptr)
			}
		case *dns.CNAME:
			tt := section[i].(*dns.CNAME)
			if x.Target != tt.Target {
				return fmt.Errorf("CNAME target should be %q, but is %q", tt.Target, x.Target)
			}
		case *dns.MX:
			tt := section[i].(*dns.MX)
			if x.Mx != tt.Mx {
				return fmt.Errorf("MX Mx should be %q, but is %q", tt.Mx, x.Mx)
			}
			if x.Preference != tt.Preference {
				return fmt.Errorf("MX Preference should be %q, but is %q", tt.Preference, x.Preference)
			}
		case *dns.NS:
			tt := section[i].(*dns.NS)
			if x.Ns != tt.Ns {
				return fmt.Errorf("NS nameserver should be %q, but is %q", tt.Ns, x.Ns)
			}
		case *dns.OPT:
			tt := section[i].(*dns.OPT)
			if x.UDPSize() != tt.UDPSize() {
				return fmt.Errorf("OPT UDPSize should be %d, but is %d", tt.UDPSize(), x.UDPSize())
			}
			if x.Do() != tt.Do() {
				return fmt.Errorf("OPT DO should be %t, but is %t", tt.Do(), x.Do())
			}
		}
	}
	return nil
}

// CNAMEOrder makes sure that CNAMES do not appear after their target records
func CNAMEOrder(res *dns.Msg) error {
	for i, c := range res.Answer {
		if c.Header().Rrtype != dns.TypeCNAME {
			continue
		}
		for _, a := range res.Answer[:i] {
			if a.Header().Name != c.(*dns.CNAME).Target {
				continue
			}
			return fmt.Errorf("CNAME found after target record")
		}
	}
	return nil
}

// SortAndCheck sorts resp and the checks the header and three sections against the testcase in tc.
func SortAndCheck(resp *dns.Msg, tc Case) error {
	sort.Sort(RRSet(resp.Answer))
	sort.Sort(RRSet(resp.Ns))
	sort.Sort(RRSet(resp.Extra))

	if err := Header(tc, resp); err != nil {
		return err
	}
	if err := Section(tc, Answer, resp.Answer); err != nil {
		return err
	}
	if err := Section(tc, Ns, resp.Ns); err != nil {
		return err

	}
	if err := Section(tc, Extra, resp.Extra); err != nil {
		return err
	}
	return nil
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
