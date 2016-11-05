package middleware

import (
	"fmt"
	"math"
	"net"
	"time"

	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

// A returns A records from Backend or an error.
func A(b ServiceBackend, zone string, state request.Request, previousRecords []dns.RR, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := b.Services(state, false, opt)
	if err != nil {
		return nil, debug, err
	}

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
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
			nextRecords, nextDebug, err := A(b, zone, state1, append(previousRecords, newRecord), opt)

			if err == nil {
				// Not only have we found something we should add the CNAME and the IP addresses.
				if len(nextRecords) > 0 {
					records = append(records, newRecord)
					records = append(records, nextRecords...)
					debug = append(debug, nextDebug...)
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
				debugMsg := msg.Service{Key: msg.Path(target, b.Debug()), Host: target, Text: " IN " + state.Type() + ": " + e1.Error()}
				debug = append(debug, debugMsg)
				continue
			}
			// Len(m1.Answer) > 0 here is well?
			records = append(records, newRecord)
			records = append(records, m1.Answer...)
			continue
		case ip.To4() != nil:
			records = append(records, serv.NewA(state.QName(), ip.To4()))
		case ip.To4() == nil:
			// nodata?
		}
	}
	return records, debug, nil
}

// AAAA returns AAAA records from Backend or an error.
func AAAA(b ServiceBackend, zone string, state request.Request, previousRecords []dns.RR, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := b.Services(state, false, opt)
	if err != nil {
		return nil, debug, err
	}

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
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
			nextRecords, nextDebug, err := AAAA(b, zone, state1, append(previousRecords, newRecord), opt)

			if err == nil {
				// Not only have we found something we should add the CNAME and the IP addresses.
				if len(nextRecords) > 0 {
					records = append(records, newRecord)
					records = append(records, nextRecords...)
					debug = append(debug, nextDebug...)
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
				debugMsg := msg.Service{Key: msg.Path(target, b.Debug()), Host: target, Text: " IN " + state.Type() + ": " + e1.Error()}
				debug = append(debug, debugMsg)
				continue
			}
			// Len(m1.Answer) > 0 here is well?
			records = append(records, newRecord)
			records = append(records, m1.Answer...)
			continue
			// both here again
		case ip.To4() != nil:
			// nada?
		case ip.To4() == nil:
			records = append(records, serv.NewAAAA(state.QName(), ip.To16()))
		}
	}
	return records, debug, nil
}

// SRV returns SRV records from the Backend.
// If the Target is not a name but an IP address, a name is created on the fly.
func SRV(b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, debug []msg.Service, err error) {
	services, debug, err := b.Services(state, false, opt)
	if err != nil {
		return nil, nil, nil, err
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
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
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
				} else {
					debugMsg := msg.Service{Key: msg.Path(srv.Target, b.Debug()), Host: srv.Target, Text: " IN A: " + e1.Error()}
					debug = append(debug, debugMsg)
				}

				m1, e1 = b.Lookup(state, srv.Target, dns.TypeAAAA)
				if e1 == nil {
					// If we have seen CNAME's we *assume* that they are already added.
					for _, a := range m1.Answer {
						if _, ok := a.(*dns.CNAME); !ok {
							extra = append(extra, a)
						}
					}
				} else {
					debugMsg := msg.Service{Key: msg.Path(srv.Target, b.Debug()), Host: srv.Target, Text: " IN AAAA: " + e1.Error()}
					debug = append(debug, debugMsg)
				}
				break
			}
			// Internal name, we should have some info on them, either v4 or v6
			// Clients expect a complete answer, because we are a recursor in their view.
			state1 := state.NewWithQuestion(srv.Target, dns.TypeA)
			addr, debugAddr, e1 := A(b, zone, state1, nil, Options(opt))
			if e1 == nil {
				extra = append(extra, addr...)
				debug = append(debug, debugAddr...)
			}
			// IPv6 lookups here as well? AAAA(zone, state1, nil).
		case ip.To4() != nil:
			serv.Host = msg.Domain(serv.Key)
			srv := serv.NewSRV(state.QName(), weight)

			records = append(records, srv)
			extra = append(extra, serv.NewA(srv.Target, ip.To4()))
		case ip.To4() == nil:
			serv.Host = msg.Domain(serv.Key)
			srv := serv.NewSRV(state.QName(), weight)

			records = append(records, srv)
			extra = append(extra, serv.NewAAAA(srv.Target, ip.To16()))
		}
	}
	return records, extra, debug, nil
}

// MX returns MX records from the Backend. If the Target is not a name but an IP address, a name is created on the fly.
func MX(b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, debug []msg.Service, err error) {
	services, debug, err := b.Services(state, false, opt)
	if err != nil {
		return nil, nil, debug, err
	}

	lookup := make(map[string]bool)
	for _, serv := range services {
		if !serv.Mail {
			continue
		}
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
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
				} else {
					debugMsg := msg.Service{Key: msg.Path(mx.Mx, b.Debug()), Host: mx.Mx, Text: " IN A: " + e1.Error()}
					debug = append(debug, debugMsg)
				}
				m1, e1 = b.Lookup(state, mx.Mx, dns.TypeAAAA)
				if e1 == nil {
					// If we have seen CNAME's we *assume* that they are already added.
					for _, a := range m1.Answer {
						if _, ok := a.(*dns.CNAME); !ok {
							extra = append(extra, a)
						}
					}
				} else {
					debugMsg := msg.Service{Key: msg.Path(mx.Mx, b.Debug()), Host: mx.Mx, Text: " IN AAAA: " + e1.Error()}
					debug = append(debug, debugMsg)
				}
				break
			}
			// Internal name
			state1 := state.NewWithQuestion(mx.Mx, dns.TypeA)
			addr, debugAddr, e1 := A(b, zone, state1, nil, opt)
			if e1 == nil {
				extra = append(extra, addr...)
				debug = append(debug, debugAddr...)
			}
			// e.AAAA as well
		case ip.To4() != nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewMX(state.QName()))
			extra = append(extra, serv.NewA(serv.Host, ip.To4()))
		case ip.To4() == nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewMX(state.QName()))
			extra = append(extra, serv.NewAAAA(serv.Host, ip.To16()))
		}
	}
	return records, extra, debug, nil
}

// CNAME returns CNAME records from the backend or an error.
func CNAME(b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := b.Services(state, true, opt)
	if err != nil {
		return nil, debug, err
	}

	if len(services) > 0 {
		serv := services[0]
		if ip := net.ParseIP(serv.Host); ip == nil {
			records = append(records, serv.NewCNAME(state.QName(), serv.Host))
		}
	}
	return records, debug, nil
}

// TXT returns TXT records from Backend or an error.
func TXT(b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := b.Services(state, false, opt)
	if err != nil {
		return nil, debug, err
	}

	for _, serv := range services {
		if serv.Text == "" {
			continue
		}
		records = append(records, serv.NewTXT(state.QName()))
	}
	return records, debug, nil
}

// PTR returns the PTR records from the backend, only services that have a domain name as host are included.
func PTR(b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := b.Reverse(state, true, opt)
	if err != nil {
		return nil, debug, err
	}

	for _, serv := range services {
		if ip := net.ParseIP(serv.Host); ip == nil {
			records = append(records, serv.NewPTR(state.QName(), serv.Host))
		}
	}
	return records, debug, nil
}

// NS returns NS records from  the backend
func NS(b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, debug []msg.Service, err error) {
	// NS record for this zone live in a special place, ns.dns.<zone>. Fake our lookup.
	// only a tad bit fishy...
	old := state.QName()

	state.Clear()
	state.Req.Question[0].Name = "ns.dns." + zone
	services, debug, err := b.Services(state, false, opt)
	if err != nil {
		return nil, nil, debug, err
	}
	// ... and reset
	state.Req.Question[0].Name = old

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			return nil, nil, debug, fmt.Errorf("NS record must be an IP address: %s", serv.Host)
		case ip.To4() != nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewNS(state.QName()))
			extra = append(extra, serv.NewA(serv.Host, ip.To4()))
		case ip.To4() == nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewNS(state.QName()))
			extra = append(extra, serv.NewAAAA(serv.Host, ip.To16()))
		}
	}
	return records, extra, debug, nil
}

// SOA returns a SOA record from the backend.
func SOA(b ServiceBackend, zone string, state request.Request, opt Options) ([]dns.RR, []msg.Service, error) {
	header := dns.RR_Header{Name: zone, Rrtype: dns.TypeSOA, Ttl: 300, Class: dns.ClassINET}

	soa := &dns.SOA{Hdr: header,
		Mbox:    hostmaster + "." + zone,
		Ns:      "ns.dns." + zone,
		Serial:  uint32(time.Now().Unix()),
		Refresh: 7200,
		Retry:   1800,
		Expire:  86400,
		Minttl:  minTTL,
	}
	// TODO(miek): fake some msg.Service here when returning?
	return []dns.RR{soa}, nil, nil
}

// BackendError writes an error response to the client.
func BackendError(b ServiceBackend, zone string, rcode int, state request.Request, debug []msg.Service, err error, opt Options) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Ns, _, _ = SOA(b, zone, state, opt)
	if opt.Debug != "" {
		m.Extra = ServicesToTxt(debug)
		txt := ErrorToTxt(err)
		if txt != nil {
			m.Extra = append(m.Extra, ErrorToTxt(err))
		}
	}
	state.SizeAndDo(m)
	state.W.WriteMsg(m)
	// Return success as the rcode to signal we have written to the client.
	return dns.RcodeSuccess, nil
}

// ServicesToTxt puts debug in TXT RRs.
func ServicesToTxt(debug []msg.Service) []dns.RR {
	if debug == nil {
		return nil
	}

	rr := make([]dns.RR, len(debug))
	for i, d := range debug {
		rr[i] = d.RR()
	}
	return rr
}

// ErrorToTxt puts in error's text into an TXT RR.
func ErrorToTxt(err error) dns.RR {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if len(msg) > 255 {
		msg = msg[:255]
	}
	t := new(dns.TXT)
	t.Hdr.Class = dns.ClassCHAOS
	t.Hdr.Ttl = 0
	t.Hdr.Rrtype = dns.TypeTXT
	t.Hdr.Name = "."

	t.Txt = []string{msg}
	return t
}

const (
	minTTL     = 60
	hostmaster = "hostmaster"
)
