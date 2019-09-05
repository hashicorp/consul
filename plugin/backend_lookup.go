package plugin

import (
	"context"
	"fmt"
	"math"
	"net"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// A returns A records from Backend or an error.
func A(ctx context.Context, b ServiceBackend, zone string, state request.Request, previousRecords []dns.RR, opt Options) (records []dns.RR, err error) {
	services, err := checkForApex(ctx, b, zone, state, opt)
	if err != nil {
		return nil, err
	}

	dup := make(map[string]struct{})

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
			if dns.IsSubDomain(zone, dns.Fqdn(serv.Host)) {
				state1 := state.NewWithQuestion(serv.Host, state.QType())
				state1.Zone = zone
				nextRecords, err := A(ctx, b, zone, state1, append(previousRecords, newRecord), opt)

				if err == nil {
					// Not only have we found something we should add the CNAME and the IP addresses.
					if len(nextRecords) > 0 {
						records = append(records, newRecord)
						records = append(records, nextRecords...)
					}
				}
				continue
			}
			// This means we can not complete the CNAME, try to look else where.
			target := newRecord.Target
			// Lookup
			m1, e1 := b.Lookup(ctx, state, target, state.QType())
			if e1 != nil {
				continue
			}
			// Len(m1.Answer) > 0 here is well?
			records = append(records, newRecord)
			records = append(records, m1.Answer...)
			continue

		case dns.TypeA:
			if _, ok := dup[serv.Host]; !ok {
				dup[serv.Host] = struct{}{}
				records = append(records, serv.NewA(state.QName(), ip))
			}

		case dns.TypeAAAA:
			// nada
		}
	}
	return records, nil
}

// AAAA returns AAAA records from Backend or an error.
func AAAA(ctx context.Context, b ServiceBackend, zone string, state request.Request, previousRecords []dns.RR, opt Options) (records []dns.RR, err error) {
	services, err := checkForApex(ctx, b, zone, state, opt)
	if err != nil {
		return nil, err
	}

	dup := make(map[string]struct{})

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
			if dns.IsSubDomain(zone, dns.Fqdn(serv.Host)) {
				state1 := state.NewWithQuestion(serv.Host, state.QType())
				state1.Zone = zone
				nextRecords, err := AAAA(ctx, b, zone, state1, append(previousRecords, newRecord), opt)

				if err == nil {
					// Not only have we found something we should add the CNAME and the IP addresses.
					if len(nextRecords) > 0 {
						records = append(records, newRecord)
						records = append(records, nextRecords...)
					}
				}
				continue
			}
			// This means we can not complete the CNAME, try to look else where.
			target := newRecord.Target
			m1, e1 := b.Lookup(ctx, state, target, state.QType())
			if e1 != nil {
				continue
			}
			// Len(m1.Answer) > 0 here is well?
			records = append(records, newRecord)
			records = append(records, m1.Answer...)
			continue
			// both here again

		case dns.TypeA:
			// nada

		case dns.TypeAAAA:
			if _, ok := dup[serv.Host]; !ok {
				dup[serv.Host] = struct{}{}
				records = append(records, serv.NewAAAA(state.QName(), ip))
			}
		}
	}
	return records, nil
}

// SRV returns SRV records from the Backend.
// If the Target is not a name but an IP address, a name is created on the fly.
func SRV(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, err error) {
	services, err := b.Services(ctx, state, false, opt)
	if err != nil {
		return nil, nil, err
	}

	dup := make(map[item]struct{})
	lookup := make(map[string]struct{})

	// Looping twice to get the right weight vs priority. This might break because we may drop duplicate SRV records latter on.
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
	for _, serv := range services {
		// Don't add the entry if the port is -1 (invalid). The kubernetes plugin uses port -1 when a service/endpoint
		// does not have any declared ports.
		if serv.Port == -1 {
			continue
		}
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

			lookup[srv.Target] = struct{}{}

			if !dns.IsSubDomain(zone, srv.Target) {
				m1, e1 := b.Lookup(ctx, state, srv.Target, dns.TypeA)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}

				m1, e1 = b.Lookup(ctx, state, srv.Target, dns.TypeAAAA)
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
			addr, e1 := A(ctx, b, zone, state1, nil, opt)
			if e1 == nil {
				extra = append(extra, addr...)
			}
			// TODO(miek): AAAA as well here.

		case dns.TypeA, dns.TypeAAAA:
			addr := serv.Host
			serv.Host = msg.Domain(serv.Key)
			srv := serv.NewSRV(state.QName(), weight)

			if ok := isDuplicate(dup, srv.Target, "", srv.Port); !ok {
				records = append(records, srv)
			}

			if ok := isDuplicate(dup, srv.Target, addr, 0); !ok {
				extra = append(extra, newAddress(serv, srv.Target, ip, what))
			}
		}
	}
	return records, extra, nil
}

// MX returns MX records from the Backend. If the Target is not a name but an IP address, a name is created on the fly.
func MX(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, err error) {
	services, err := b.Services(ctx, state, false, opt)
	if err != nil {
		return nil, nil, err
	}

	dup := make(map[item]struct{})
	lookup := make(map[string]struct{})
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

			lookup[mx.Mx] = struct{}{}

			if !dns.IsSubDomain(zone, mx.Mx) {
				m1, e1 := b.Lookup(ctx, state, mx.Mx, dns.TypeA)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}

				m1, e1 = b.Lookup(ctx, state, mx.Mx, dns.TypeAAAA)
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
			addr, e1 := A(ctx, b, zone, state1, nil, opt)
			if e1 == nil {
				extra = append(extra, addr...)
			}
			// TODO(miek): AAAA as well here.

		case dns.TypeA, dns.TypeAAAA:
			addr := serv.Host
			serv.Host = msg.Domain(serv.Key)
			mx := serv.NewMX(state.QName())

			if ok := isDuplicate(dup, mx.Mx, "", mx.Preference); !ok {
				records = append(records, mx)
			}
			// Fake port to be 0 for address...
			if ok := isDuplicate(dup, serv.Host, addr, 0); !ok {
				extra = append(extra, newAddress(serv, serv.Host, ip, what))
			}
		}
	}
	return records, extra, nil
}

// CNAME returns CNAME records from the backend or an error.
func CNAME(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, err error) {
	services, err := b.Services(ctx, state, true, opt)
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
func TXT(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, err error) {
	services, err := b.Services(ctx, state, false, opt)
	if err != nil {
		return nil, err
	}

	for _, serv := range services {
		records = append(records, serv.NewTXT(state.QName()))
	}
	return records, nil
}

// PTR returns the PTR records from the backend, only services that have a domain name as host are included.
func PTR(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) (records []dns.RR, err error) {
	services, err := b.Reverse(ctx, state, true, opt)
	if err != nil {
		return nil, err
	}

	dup := make(map[string]struct{})

	for _, serv := range services {
		if ip := net.ParseIP(serv.Host); ip == nil {
			if _, ok := dup[serv.Host]; !ok {
				dup[serv.Host] = struct{}{}
				records = append(records, serv.NewPTR(state.QName(), serv.Host))
			}
		}
	}
	return records, nil
}

// NS returns NS records from  the backend
func NS(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) (records, extra []dns.RR, err error) {
	// NS record for this zone live in a special place, ns.dns.<zone>. Fake our lookup.
	// only a tad bit fishy...
	old := state.QName()

	state.Clear()
	state.Req.Question[0].Name = "ns.dns." + zone
	services, err := b.Services(ctx, state, false, opt)
	if err != nil {
		return nil, nil, err
	}
	// ... and reset
	state.Req.Question[0].Name = old

	seen := map[string]bool{}

	for _, serv := range services {
		what, ip := serv.HostType()
		switch what {
		case dns.TypeCNAME:
			return nil, nil, fmt.Errorf("NS record must be an IP address: %s", serv.Host)

		case dns.TypeA, dns.TypeAAAA:
			serv.Host = msg.Domain(serv.Key)
			extra = append(extra, newAddress(serv, serv.Host, ip, what))
			ns := serv.NewNS(state.QName())
			if _, ok := seen[ns.Ns]; ok {
				continue
			}
			seen[ns.Ns] = true
			records = append(records, ns)
		}
	}
	return records, extra, nil
}

// SOA returns a SOA record from the backend.
func SOA(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) ([]dns.RR, error) {
	minTTL := b.MinTTL(state)
	ttl := uint32(300)
	if minTTL < ttl {
		ttl = minTTL
	}

	header := dns.RR_Header{Name: zone, Rrtype: dns.TypeSOA, Ttl: ttl, Class: dns.ClassINET}

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
		Minttl:  minTTL,
	}
	return []dns.RR{soa}, nil
}

// BackendError writes an error response to the client.
func BackendError(ctx context.Context, b ServiceBackend, zone string, rcode int, state request.Request, err error, opt Options) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative = true
	m.Ns, _ = SOA(ctx, b, zone, state, opt)

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

// checkForApex checks the special apex.dns directory for records that will be returned as A or AAAA.
func checkForApex(ctx context.Context, b ServiceBackend, zone string, state request.Request, opt Options) ([]msg.Service, error) {
	if state.Name() != zone {
		return b.Services(ctx, state, false, opt)
	}

	// If the zone name itself is queried we fake the query to search for a special entry
	// this is equivalent to the NS search code.
	old := state.QName()
	state.Clear()
	state.Req.Question[0].Name = dnsutil.Join("apex.dns", zone)

	services, err := b.Services(ctx, state, false, opt)
	if err == nil {
		state.Req.Question[0].Name = old
		return services, err
	}

	state.Req.Question[0].Name = old
	return b.Services(ctx, state, false, opt)
}

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

const hostmaster = "hostmaster"
