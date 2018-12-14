package external

import (
	"math"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func (e *External) a(services []msg.Service, state request.Request) (records []dns.RR) {
	dup := make(map[string]struct{})

	for _, s := range services {

		what, ip := s.HostType()

		switch what {
		case dns.TypeCNAME:
			// can't happen

		case dns.TypeA:
			if _, ok := dup[s.Host]; !ok {
				dup[s.Host] = struct{}{}
				rr := s.NewA(state.QName(), ip)
				rr.Hdr.Ttl = e.ttl
				records = append(records, rr)
			}

		case dns.TypeAAAA:
			// nada
		}
	}
	return records
}

func (e *External) aaaa(services []msg.Service, state request.Request) (records []dns.RR) {
	dup := make(map[string]struct{})

	for _, s := range services {

		what, ip := s.HostType()

		switch what {
		case dns.TypeCNAME:
			// can't happen

		case dns.TypeA:
			// nada

		case dns.TypeAAAA:
			if _, ok := dup[s.Host]; !ok {
				dup[s.Host] = struct{}{}
				rr := s.NewAAAA(state.QName(), ip)
				rr.Hdr.Ttl = e.ttl
				records = append(records, rr)
			}
		}
	}
	return records
}

func (e *External) srv(services []msg.Service, state request.Request) (records, extra []dns.RR) {
	dup := make(map[item]struct{})

	// Looping twice to get the right weight vs priority. This might break because we may drop duplicate SRV records latter on.
	w := make(map[int]int)
	for _, s := range services {
		weight := 100
		if s.Weight != 0 {
			weight = s.Weight
		}
		if _, ok := w[s.Priority]; !ok {
			w[s.Priority] = weight
			continue
		}
		w[s.Priority] += weight
	}
	for _, s := range services {
		// Don't add the entry if the port is -1 (invalid). The kubernetes plugin uses port -1 when a service/endpoint
		// does not have any declared ports.
		if s.Port == -1 {
			continue
		}
		w1 := 100.0 / float64(w[s.Priority])
		if s.Weight == 0 {
			w1 *= 100
		} else {
			w1 *= float64(s.Weight)
		}
		weight := uint16(math.Floor(w1))

		what, ip := s.HostType()

		switch what {
		case dns.TypeCNAME:
			// can't happen

		case dns.TypeA, dns.TypeAAAA:
			addr := s.Host
			s.Host = msg.Domain(s.Key)
			srv := s.NewSRV(state.QName(), weight)

			if ok := isDuplicate(dup, srv.Target, "", srv.Port); !ok {
				records = append(records, srv)
			}

			if ok := isDuplicate(dup, srv.Target, addr, 0); !ok {
				hdr := dns.RR_Header{Name: srv.Target, Rrtype: what, Class: dns.ClassINET, Ttl: e.ttl}

				switch what {
				case dns.TypeA:
					extra = append(extra, &dns.A{Hdr: hdr, A: ip})
				case dns.TypeAAAA:
					extra = append(extra, &dns.AAAA{Hdr: hdr, AAAA: ip})
				}
			}
		}
	}
	return records, extra
}

// not sure if this is even needed.

// item holds records.
type item struct {
	name string // name of the record (either owner or something else unique).
	port uint16 // port of the record (used for address records, A and AAAA).
	addr string // address of the record (A and AAAA).
}

// isDuplicate uses m to see if the combo (name, addr, port) already exists. If it does
// not exist already IsDuplicate will also add the record to the map.
func isDuplicate(m map[item]struct{}, name, addr string, port uint16) bool {
	if addr != "" {
		_, ok := m[item{name, 0, addr}]
		if !ok {
			m[item{name, 0, addr}] = struct{}{}
		}
		return ok
	}
	_, ok := m[item{name, port, ""}]
	if !ok {
		m[item{name, port, ""}] = struct{}{}
	}
	return ok
}
