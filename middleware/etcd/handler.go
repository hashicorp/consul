package etcd

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (e Etcd) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	println("ETCD MIDDLEWARE HIT")

	state := middleware.State{W: w, Req: r}

	m := state.AnswerMessage()
	m.Authoritative = true
	m.RecursionAvailable = true
	m.Compress = true

	return 0, nil
}

// only needs state and current zone name we are auth for.
/*
func (s *server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {

	q := req.Question[0]
	name := strings.ToLower(q.Name)

	switch q.Qtype {
	case dns.TypeNS:
		records, extra, err := s.NSRecords(q, s.config.dnsDomain)
		if isEtcdNameError(err, s) {
			m = s.NameError(req)
			return
		}
		m.Answer = append(m.Answer, records...)
		m.Extra = append(m.Extra, extra...)
	case dns.TypeA, dns.TypeAAAA:
		records, err := s.AddressRecords(q, name, nil, bufsize, dnssec, false)
		if isEtcdNameError(err, s) {
			m = s.NameError(req)
			return
		}
		m.Answer = append(m.Answer, records...)
	case dns.TypeTXT:
		records, err := s.TXTRecords(q, name)
		if isEtcdNameError(err, s) {
			m = s.NameError(req)
			return
		}
		m.Answer = append(m.Answer, records...)
	case dns.TypeCNAME:
		records, err := s.CNAMERecords(q, name)
		if isEtcdNameError(err, s) {
			m = s.NameError(req)
			return
		}
		m.Answer = append(m.Answer, records...)
	case dns.TypeMX:
		records, extra, err := s.MXRecords(q, name, bufsize, dnssec)
		if isEtcdNameError(err, s) {
			m = s.NameError(req)
			return
		}
		m.Answer = append(m.Answer, records...)
		m.Extra = append(m.Extra, extra...)
	default:
		fallthrough // also catch other types, so that they return NODATA
	case dns.TypeSRV:
		records, extra, err := s.SRVRecords(q, name, bufsize, dnssec)
		if err != nil {
			if isEtcdNameError(err, s) {
				m = s.NameError(req)
				return
			}
			logf("got error from backend: %s", err)
			if q.Qtype == dns.TypeSRV { // Otherwise NODATA
				m = s.ServerFailure(req)
				return
			}
		}
		// if we are here again, check the types, because an answer may only
		// be given for SRV. All other types should return NODATA, the
		// NXDOMAIN part is handled in the above code. TODO(miek): yes this
		// can be done in a more elegant manor.
		if q.Qtype == dns.TypeSRV {
			m.Answer = append(m.Answer, records...)
			m.Extra = append(m.Extra, extra...)
		}
	}

	if len(m.Answer) == 0 { // NODATA response
		m.Ns = []dns.RR{s.NewSOA()}
		m.Ns[0].Header().Ttl = s.config.MinTtl
	}
}

// etcNameError checks if the error is ErrorCodeKeyNotFound from etcd.
func isEtcdNameError(err error, s *server) bool {
	if e, ok := err.(etcd.Error); ok && e.Code == etcd.ErrorCodeKeyNotFound {
		return true
	}
	return false
}
*/
