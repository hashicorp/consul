package file

import (
	"io"
	"log"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type (
	File struct {
		Next  middleware.Handler
		Zones Zones
	}

	Zones struct {
		Z     map[string]*Zone
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
	z, ok := f.Zones.Z[zone]
	if !ok {
		return f.Next.ServeDNS(ctx, w, r)
	}

	if state.Proto() != "udp" && state.QType() == dns.TypeAXFR {
		xfr := Xfr{z}
		return xfr.ServeDNS(ctx, w, r)
	}

	rrs, extra, result := z.Lookup(qname, state.QType(), state.Do())

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	switch result {
	case Success:
		// case?
		m.Answer = rrs
		m.Extra = extra
		// Ns section
	case NameError:
		m.Rcode = dns.RcodeNameError
		fallthrough
	case NoData:
		// case?
		m.Ns = rrs
	default:
		// TODO
	}
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Parse parses the zone in filename and returns a new Zone or an error.
func Parse(f io.Reader, origin, fileName string) (*Zone, error) {
	tokens := dns.ParseZone(f, dns.Fqdn(origin), fileName)
	z := NewZone(origin)
	for x := range tokens {
		if x.Error != nil {
			log.Printf("[ERROR] failed to parse %s: %v", origin, x.Error)
			return nil, x.Error
		}
		if x.RR.Header().Rrtype == dns.TypeSOA {
			z.SOA = x.RR.(*dns.SOA)
			continue
		}
		if x.RR.Header().Rrtype == dns.TypeRRSIG {
			if x, ok := x.RR.(*dns.RRSIG); ok && x.TypeCovered == dns.TypeSOA {
				z.SIG = append(z.SIG, x)
			}
		}
		z.Insert(x.RR)
	}
	return z, nil
}
