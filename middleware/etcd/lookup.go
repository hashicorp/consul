package etcd

import (
	"fmt"
	"math"
	"net"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"

	"github.com/miekg/dns"
)

type Options struct {
	Debug string
}

func (e Etcd) records(state middleware.State, exact bool, opt Options) (services, debug []msg.Service, err error) {
	services, err = e.Records(state.Name(), exact)
	if err != nil {
		return
	}
	if opt.Debug != "" {
		debug = services
	}
	services = msg.Group(services)
	return
}

func (e Etcd) A(zone string, state middleware.State, previousRecords []dns.RR, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := e.records(state, false, opt)
	if err != nil {
		return nil, debug, err
	}

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			// TODO(miek): lowercasing? Should lowercase in everything see #85
			if middleware.Name(state.Name()).Matches(dns.Fqdn(serv.Host)) {
				// x CNAME x is a direct loop, don't add those
				continue
			}

			newRecord := serv.NewCNAME(state.QName(), serv.Host)
			if len(previousRecords) > 7 {
				// don't add it, and just continue
				continue
			}
			if isDuplicateCNAME(newRecord, previousRecords) {
				continue
			}

			state1 := copyState(state, serv.Host, state.QType())
			nextRecords, nextDebug, err := e.A(zone, state1, append(previousRecords, newRecord), opt)

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
			m1, e1 := e.Proxy.Lookup(state, target, state.QType())
			if e1 != nil {
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

func (e Etcd) AAAA(zone string, state middleware.State, previousRecords []dns.RR, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := e.records(state, false, opt)
	if err != nil {
		return nil, debug, err
	}

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			// Try to resolve as CNAME if it's not an IP, but only if we don't create loops.
			if middleware.Name(state.Name()).Matches(dns.Fqdn(serv.Host)) {
				// x CNAME x is a direct loop, don't add those
				continue
			}

			newRecord := serv.NewCNAME(state.QName(), serv.Host)
			if len(previousRecords) > 7 {
				// don't add it, and just continue
				continue
			}
			if isDuplicateCNAME(newRecord, previousRecords) {
				continue
			}

			state1 := copyState(state, serv.Host, state.QType())
			nextRecords, nextDebug, err := e.AAAA(zone, state1, append(previousRecords, newRecord), opt)

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
			m1, e1 := e.Proxy.Lookup(state, target, state.QType())
			if e1 != nil {
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

// SRV returns SRV records from etcd.
// If the Target is not a name but an IP address, a name is created on the fly.
func (e Etcd) SRV(zone string, state middleware.State, opt Options) (records, extra []dns.RR, debug []msg.Service, err error) {
	services, debug, err := e.records(state, false, opt)
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
				m1, e1 := e.Proxy.Lookup(state, srv.Target, dns.TypeA)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}
				m1, e1 = e.Proxy.Lookup(state, srv.Target, dns.TypeAAAA)
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
			state1 := copyState(state, srv.Target, dns.TypeA)
			addr, debugAddr, e1 := e.A(zone, state1, nil, opt)
			if e1 == nil {
				extra = append(extra, addr...)
				debug = append(debug, debugAddr...)
			}
			// e.AAA(zone, state1, nil) as well...?
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

// MX returns MX records from etcd.
// If the Target is not a name but an IP address, a name is created on the fly.
func (e Etcd) MX(zone string, state middleware.State, opt Options) (records, extra []dns.RR, debug []msg.Service, err error) {
	services, debug, err := e.records(state, false, opt)
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
				m1, e1 := e.Proxy.Lookup(state, mx.Mx, dns.TypeA)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}
				m1, e1 = e.Proxy.Lookup(state, mx.Mx, dns.TypeAAAA)
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
			state1 := copyState(state, mx.Mx, dns.TypeA)
			addr, debugAddr, e1 := e.A(zone, state1, nil, opt)
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

func (e Etcd) CNAME(zone string, state middleware.State, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := e.records(state, true, opt)
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

// PTR returns the PTR records, only services that have a domain name as host are included.
func (e Etcd) PTR(zone string, state middleware.State, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := e.records(state, true, opt)
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

func (e Etcd) TXT(zone string, state middleware.State, opt Options) (records []dns.RR, debug []msg.Service, err error) {
	services, debug, err := e.records(state, false, opt)
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

func (e Etcd) NS(zone string, state middleware.State, opt Options) (records, extra []dns.RR, debug []msg.Service, err error) {
	// NS record for this zone live in a special place, ns.dns.<zone>. Fake our lookup.
	// only a tad bit fishy...
	old := state.QName()

	state.Clear()
	state.Req.Question[0].Name = "ns.dns." + zone
	services, debug, err := e.records(state, false, opt)
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

// SOA Record returns a SOA record.
func (e Etcd) SOA(zone string, state middleware.State, opt Options) ([]dns.RR, []msg.Service, error) {
	header := dns.RR_Header{Name: zone, Rrtype: dns.TypeSOA, Ttl: 300, Class: dns.ClassINET}

	soa := &dns.SOA{Hdr: header,
		Mbox:    "hostmaster." + zone,
		Ns:      "ns.dns." + zone,
		Serial:  uint32(time.Now().Unix()),
		Refresh: 7200,
		Retry:   1800,
		Expire:  86400,
		Minttl:  60,
	}
	// TODO(miek): fake some msg.Service here when returning.
	return []dns.RR{soa}, nil, nil
}

func isDuplicateCNAME(r *dns.CNAME, records []dns.RR) bool {
	for _, rec := range records {
		if v, ok := rec.(*dns.CNAME); ok {
			if v.Target == r.Target {
				return true
			}
		}
	}
	return false
}

// TODO(miek): Move to middleware?
func copyState(state middleware.State, target string, typ uint16) middleware.State {
	state1 := middleware.State{W: state.W, Req: state.Req.Copy()}
	state1.Req.Question[0] = dns.Question{Name: dns.Fqdn(target), Qclass: dns.ClassINET, Qtype: typ}
	return state1
}
