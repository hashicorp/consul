package plugin

import (
	"fmt"
	"math"
	"net"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// A returns A records from Backend or an error.
func A(b ServiceBackend, zone string, state request.Request, previousRecords []dns.RR, opt Options) (records []dns.RR, err error) {
	services, err := b.Services(state, false, opt)
	if err != nil {
		return nil, err
	}

	for _, serv := range services {

		what, ip := serv.HostType()

		switch what {
		case dns.TypeCNAME:
			if Name(state.Name()).Matches(dns.Fqdn(serv.Host)) {
				// x CNAME x is a direct loop, don't add those
				continue
			}

			newRecord := serv.NewCNAME(state.QName(), serv.Host)
			if len(previousRecords) > 7 {
				// don't add it, and just continue
				continue
			}
			if dnsutil.DuplicateCNAME(newRecord, previousRecords) {
				continue
			}

			state1 := state.NewWithQuestion(serv.Host, state.QType())
			nextRecords, err := A(b, zone, state1, append(previousRecords, newRecord), opt)

			if err == nil {
				// Not only have we found something we should add the CNAME and the IP addresses.
				if len(nextRecords) > 0 {
					records = append(records, newRecord)
					records = append(records, nextRecords...)
				}
				continue
			}
			// This means we can not complete the CNAME, try to look else where.
			target := newRecord.Target
			if dns.IsSubDomain(zone, target) {
				// We should already have found it
				continue
			}
			// Lookup
			m1, e1 := b.Lookup(state, target, state.QType())
			if e1 != nil {
				continue
			}
			// Len(m1.Answer) > 0 here is well?
			records = append(records, newRecord)
			records = append(records, m1.Answer...)
			continue

		case dns.TypeA:
			records = append(records, serv.NewA(state.QName(), ip))

		case dns.TypeAAAA:
			// nodata?
		}
	}
	return records, nil
}

// AAAA returns AAAA records from Backend or an error.
func AAAA(b ServiceBackend, zone string, state request.Request, previousRecords []dns.RR, opt Options) (records []dns.RR, err error) {
	services, err := b.Services(state, false, opt)
	if err != nil {
		return nil, err
	}

	for _, serv := range services {

		what, ip := serv.HostType()

		switch what {
		case dns.TypeCNAME:
			// Try to resolve as CNAME if it's not an IP, but only if we don't create loops.
			if Name(state.Name()).Matches(dns.Fqdn(serv.Host)) {
				// x CNAME x is a direct loop, don't add those
				continue
			}

			newRecord := serv.NewCNAME(state.QName(), serv.Host)
			if len(previousRecords) > 7 {
				// don't add it, and just continue
				continue
			}
			if dnsutil.DuplicateCNAME(newRecord, previousRecords) {
				continue
			}

			state1 := state.NewWithQuestion(serv.Host, state.QType())
			nextRecords, err := AAAA(b, zone, state1, append(previousRecords, newRecord), opt)

			if err == nil {
				// Not only have we found something we should add the CNAME and the IP addresses.
				if len(nextRecords) > 0 {
					records = append(records, newRecord)
					records = append(records, nextRecords...)
				}
				continue
			}
			// This means we can not complete the CNAME, try to look else where.
			target := newRecord.Target
			if dns.IsSubDomain(zone, target) {
				// We should already have found it
				continue
			}
			m1, e1 := b.Lookup(state, target, state.QType())
			if e1 != nil {
				continue
			}
			// Len(m1.Answer) > 0 here is well?
			records = append(records, newRecord)
			records = append(records, m1.Answer...)
			continue
			// both here again

		case dns.TypeA:
			// nada?

		case dns.TypeAAAA:
			records = append(records, serv.NewAAAA(state.QName(), ip))
		}
	}
	return records, nil
}

// SRV returns SRV records from the Backend.
// If the Target is not a name but an IP address, a name is created on the fly.
func SRV(b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, err error) {
	services, err := b.Services(state, false, opt)
	if err != nil {
		return nil, nil, err
	}

	// Looping twice to get the right weight vs priority
	w := make(map[int]int)
	for _, serv := range services {
		weight := 100
		if serv.Weight != 0 {
			weight = serv.Weight
		}
		if _, ok := w[serv.Priority]; !ok {
			w[serv.Priority] = weight
			continue
		}
		w[serv.Priority] += weight
	}
	lookup := make(map[string]bool)
	for _, serv := range services {
		w1 := 100.0 / float64(w[serv.Priority])
		if serv.Weight == 0 {
			w1 *= 100
		} else {
			w1 *= float64(serv.Weight)
		}
		weight := uint16(math.Floor(w1))

		what, ip := serv.HostType()

		switch what {
		case dns.TypeCNAME:
			srv := serv.NewSRV(state.QName(), weight)
			records = append(records, srv)

			if _, ok := lookup[srv.Target]; ok {
				break
			}

			lookup[srv.Target] = true

			if !dns.IsSubDomain(zone, srv.Target) {
				m1, e1 := b.Lookup(state, srv.Target, dns.TypeA)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}

				m1, e1 = b.Lookup(state, srv.Target, dns.TypeAAAA)
				if e1 == nil {
					// If we have seen CNAME's we *assume* that they are already added.
					for _, a := range m1.Answer {
						if _, ok := a.(*dns.CNAME); !ok {
							extra = append(extra, a)
						}
					}
				}
				break
			}
			// Internal name, we should have some info on them, either v4 or v6
			// Clients expect a complete answer, because we are a recursor in their view.
			state1 := state.NewWithQuestion(srv.Target, dns.TypeA)
			addr, e1 := A(b, zone, state1, nil, opt)
			if e1 == nil {
				extra = append(extra, addr...)
			}
			// IPv6 lookups here as well? AAAA(zone, state1, nil).

		case dns.TypeA, dns.TypeAAAA:
			serv.Host = msg.Domain(serv.Key)
			srv := serv.NewSRV(state.QName(), weight)

			records = append(records, srv)
			extra = append(extra, newAddress(serv, srv.Target, ip, what))
		}
	}
	return records, extra, nil
}

// MX returns MX records from the Backend. If the Target is not a name but an IP address, a name is created on the fly.
func MX(b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, err error) {
	services, err := b.Services(state, false, opt)
	if err != nil {
		return nil, nil, err
	}

	lookup := make(map[string]bool)
	for _, serv := range services {
		if !serv.Mail {
			continue
		}
		what, ip := serv.HostType()
		switch what {
		case dns.TypeCNAME:
			mx := serv.NewMX(state.QName())
			records = append(records, mx)
			if _, ok := lookup[mx.Mx]; ok {
				break
			}

			lookup[mx.Mx] = true

			if !dns.IsSubDomain(zone, mx.Mx) {
				m1, e1 := b.Lookup(state, mx.Mx, dns.TypeA)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}

				m1, e1 = b.Lookup(state, mx.Mx, dns.TypeAAAA)
				if e1 == nil {
					// If we have seen CNAME's we *assume* that they are already added.
					for _, a := range m1.Answer {
						if _, ok := a.(*dns.CNAME); !ok {
							extra = append(extra, a)
						}
					}
				}
				break
			}
			// Internal name
			state1 := state.NewWithQuestion(mx.Mx, dns.TypeA)
			addr, e1 := A(b, zone, state1, nil, opt)
			if e1 == nil {
				extra = append(extra, addr...)
			}
			// e.AAAA as well

		case dns.TypeA, dns.TypeAAAA:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewMX(state.QName()))
			extra = append(extra, newAddress(serv, serv.Host, ip, what))
		}
	}
	return records, extra, nil
}

// CNAME returns CNAME records from the backend or an error.
func CNAME(b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, err error) {
	services, err := b.Services(state, true, opt)
	if err != nil {
		return nil, err
	}

	if len(services) > 0 {
		serv := services[0]
		if ip := net.ParseIP(serv.Host); ip == nil {
			records = append(records, serv.NewCNAME(state.QName(), serv.Host))
		}
	}
	return records, nil
}

// TXT returns TXT records from Backend or an error.
func TXT(b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, err error) {
	services, err := b.Services(state, false, opt)
	if err != nil {
		return nil, err
	}

	for _, serv := range services {
		if serv.Text == "" {
			continue
		}
		records = append(records, serv.NewTXT(state.QName()))
	}
	return records, nil
}

// PTR returns the PTR records from the backend, only services that have a domain name as host are included.
func PTR(b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, err error) {
	services, err := b.Reverse(state, true, opt)
	if err != nil {
		return nil, err
	}

	for _, serv := range services {
		if ip := net.ParseIP(serv.Host); ip == nil {
			records = append(records, serv.NewPTR(state.QName(), serv.Host))
		}
	}
	return records, nil
}

// NS returns NS records from  the backend
func NS(b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, err error) {
	// NS record for this zone live in a special place, ns.dns.<zone>. Fake our lookup.
	// only a tad bit fishy...
	old := state.QName()

	state.Clear()
	state.Req.Question[0].Name = "ns.dns." + zone
	services, err := b.Services(state, false, opt)
	if err != nil {
		return nil, nil, err
	}
	// ... and reset
	state.Req.Question[0].Name = old

	for _, serv := range services {
		what, ip := serv.HostType()
		switch what {
		case dns.TypeCNAME:
			return nil, nil, fmt.Errorf("NS record must be an IP address: %s", serv.Host)

		case dns.TypeA, dns.TypeAAAA:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewNS(state.QName()))
			extra = append(extra, newAddress(serv, serv.Host, ip, what))
		}
	}
	return records, extra, nil
}

// SOA returns a SOA record from the backend.
func SOA(b ServiceBackend, zone string, state request.Request, opt Options) ([]dns.RR, error) {
	header := dns.RR_Header{Name: zone, Rrtype: dns.TypeSOA, Ttl: 300, Class: dns.ClassINET}

	Mbox := hostmaster + "."
	Ns := "ns.dns."
	if zone[0] != '.' {
		Mbox += zone
		Ns += zone
	}

	soa := &dns.SOA{Hdr: header,
		Mbox:    Mbox,
		Ns:      Ns,
		Serial:  b.Serial(state),
		Refresh: 7200,
		Retry:   1800,
		Expire:  86400,
		Minttl:  b.MinTTL(state),
	}
	return []dns.RR{soa}, nil
}

// BackendError writes an error response to the client.
func BackendError(b ServiceBackend, zone string, rcode int, state request.Request, err error, opt Options) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Ns, _ = SOA(b, zone, state, opt)

	state.SizeAndDo(m)
	state.W.WriteMsg(m)
	// Return success as the rcode to signal we have written to the client.
	return dns.RcodeSuccess, err
}

func newAddress(s msg.Service, name string, ip net.IP, what uint16) dns.RR {

	hdr := dns.RR_Header{Name: name, Rrtype: what, Class: dns.ClassINET, Ttl: s.TTL}

	if what == dns.TypeA {
		return &dns.A{Hdr: hdr, A: ip}
	}
	// Should always be dns.TypeAAAA
	return &dns.AAAA{Hdr: hdr, AAAA: ip}
}

const hostmaster = "hostmaster"
