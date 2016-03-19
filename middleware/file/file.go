package file

// TODO(miek): the zone's implementation is basically non-existent
// we return a list and when searching for an answer we iterate
// over the list. This must be moved to a tree-like structure and
// have some fluff for DNSSEC (and be memory efficient).

import (
	"strings"

	"golang.org/x/net/context"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

type (
	File struct {
		Next  middleware.Handler
		Zones Zones
		// Maybe a list of all zones as well, as a []string?
	}

	Zone  []dns.RR
	Zones struct {
		Z     map[string]Zone // utterly braindead impl. TODO(miek): fix
		Names []string
	}
)

func (f File) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}
	qname := state.Name()
	zone := middleware.Zones(f.Zones.Names).Matches(qname)
	if zone == "" {
		return f.Next.ServeDNS(ctx, w, r)
	}

	names, nodata := f.Zones.Z[zone].lookup(qname, state.QType())
	var answer *dns.Msg
	switch {
	case nodata:
		answer = state.AnswerMessage()
		answer.Ns = names
	case len(names) == 0:
		answer = state.AnswerMessage()
		answer.Ns = names
		answer.Rcode = dns.RcodeNameError
	case len(names) > 0:
		answer = state.AnswerMessage()
		answer.Answer = names
	default:
		answer = state.ErrorMessage(dns.RcodeServerFailure)
	}
	// Check return size, etc. TODO(miek)
	w.WriteMsg(answer)
	return 0, nil
}

// Lookup will try to find qname and qtype in z. It returns the
// records found *or* a boolean saying NODATA. If the answer
// is NODATA then the RR returned is the SOA record.
//
// TODO(miek): EXTREMELY STUPID IMPLEMENTATION.
// Doesn't do much, no delegation, no cname, nothing really, etc.
// TODO(miek): even NODATA looks broken
func (z Zone) lookup(qname string, qtype uint16) ([]dns.RR, bool) {
	var (
		nodata bool
		rep    []dns.RR
		soa    dns.RR
	)

	for _, rr := range z {
		if rr.Header().Rrtype == dns.TypeSOA {
			soa = rr
		}
		// Match function in Go DNS?
		if strings.ToLower(rr.Header().Name) == qname {
			if rr.Header().Rrtype == qtype {
				rep = append(rep, rr)
				nodata = false
			}

		}
	}
	if nodata {
		return []dns.RR{soa}, true
	}
	return rep, false
}
