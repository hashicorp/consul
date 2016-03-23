package etcd

import (
	"math"
	"net"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"

	"github.com/miekg/dns"
)

// TODO(miek): factor out common code a bit

func (e Etcd) A(zone string, state middleware.State, previousRecords []dns.RR) (records []dns.RR, err error) {
	services, err := e.Records(state.Name(), false)
	if err != nil {
		return nil, err
	}

	services = msg.Group(services)

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			// Try to resolve as CNAME if it's not an IP, but only if we don't create loops.
			// TODO(miek): lowercasing, use Match in middleware?
			if state.Name() == dns.Fqdn(serv.Host) {
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
			nextRecords, err := e.A(zone, state1, append(previousRecords, newRecord))

			if err == nil {
				// Not only have we found something we should add the CNAME and the IP addresses.
				if len(nextRecords) > 0 {
					// TODO(miek): sorting here?
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
			// noda?
		}
	}
	return records, nil
}

func (e Etcd) AAAA(zone string, state middleware.State, previousRecords []dns.RR) (records []dns.RR, err error) {
	services, err := e.Records(state.Name(), false)
	if err != nil {
		return nil, err
	}

	services = msg.Group(services)

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			// Try to resolve as CNAME if it's not an IP, but only if we don't create loops.
			// TODO(miek): lowercasing, use Match in middleware/
			if state.Name() == dns.Fqdn(serv.Host) {
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
			nextRecords, err := e.AAAA(zone, state1, append(previousRecords, newRecord))

			if err == nil {
				// Not only have we found something we should add the CNAME and the IP addresses.
				if len(nextRecords) > 0 {
					// TODO(miek): sorting here?
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
	return records, nil
}

// SRV returns SRV records from etcd.
// If the Target is not a name but an IP address, a name is created on the fly.
func (e Etcd) SRV(zone string, state middleware.State) (records []dns.RR, extra []dns.RR, err error) {
	services, err := e.Records(state.Name(), false)
	if err != nil {
		return nil, nil, err
	}

	services = msg.Group(services)

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
			// Clients expect a complete answer, because we are a recursor in their
			// view.
			state1 := copyState(state, srv.Target, dns.TypeA)
			// TODO(both is true here!
			addr, e1 := e.A(zone, state1, nil)
			if e1 == nil {
				extra = append(extra, addr...)
			}
			// e.AAA(zone, state1, nil) as well...
		case ip.To4() != nil:
			serv.Host = e.Domain(serv.Key)
			srv := serv.NewSRV(state.QName(), weight)

			records = append(records, srv)
			extra = append(extra, serv.NewA(srv.Target, ip.To4()))
		case ip.To4() == nil:
			serv.Host = e.Domain(serv.Key)
			srv := serv.NewSRV(state.QName(), weight)

			records = append(records, srv)
			extra = append(extra, serv.NewAAAA(srv.Target, ip.To16()))
		}
	}
	return records, extra, nil
}

// MX returns MX records from etcd.
// If the Target is not a name but an IP address, a name is created on the fly.
func (e Etcd) MX(zone string, state middleware.State) (records []dns.RR, extra []dns.RR, err error) {
	services, err := e.Records(state.Name(), false)
	if err != nil {
		return nil, nil, err
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
			// both is true here as well
			state1 := copyState(state, mx.Mx, dns.TypeA)
			addr, e1 := e.A(zone, state1, nil)
			if e1 == nil {
				extra = append(extra, addr...)
			}
			// e.AAAA as well
		case ip.To4() != nil:
			serv.Host = e.Domain(serv.Key)
			records = append(records, serv.NewMX(state.QName()))
			extra = append(extra, serv.NewA(serv.Host, ip.To4()))
		case ip.To4() == nil:
			serv.Host = e.Domain(serv.Key)
			records = append(records, serv.NewMX(state.QName()))
			extra = append(extra, serv.NewAAAA(serv.Host, ip.To16()))
		}
	}
	return records, extra, nil
}

func (e Etcd) CNAME(zone string, state middleware.State) (records []dns.RR, err error) {
	services, err := e.Records(state.Name(), true)
	if err != nil {
		return nil, err
	}

	services = msg.Group(services)

	if len(services) > 0 {
		serv := services[0]
		if ip := net.ParseIP(serv.Host); ip == nil {
			records = append(records, serv.NewCNAME(state.QName(), serv.Host))
		}
	}
	return records, nil
}

func (e Etcd) TXT(zone string, state middleware.State) (records []dns.RR, err error) {
	services, err := e.Records(state.Name(), false)
	if err != nil {
		return nil, err
	}

	services = msg.Group(services)

	for _, serv := range services {
		if serv.Text == "" {
			continue
		}
		records = append(records, serv.NewTXT(state.QName()))
	}
	return records, nil
}

// synthesis a SOA Record.
func SOA(zone string) *dns.SOA {
	return &dns.SOA{}
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

func copyState(state middleware.State, target string, typ uint16) middleware.State {
	state1 := state
	state1.Req.Question[0] = dns.Question{dns.Fqdn(target), dns.ClassINET, typ}
	return state1
}
