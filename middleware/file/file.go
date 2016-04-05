package file

import (
	"fmt"
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
	if state.QClass() != dns.ClassINET {
		return dns.RcodeServerFailure, fmt.Errorf("can only deal with ClassINET")
	}
	qname := state.Name()
	zone := middleware.Zones(f.Zones.Names).Matches(qname)
	if zone == "" {
		return f.Next.ServeDNS(ctx, w, r)
	}
	z, ok := f.Zones.Z[zone]
	if !ok {
		return f.Next.ServeDNS(ctx, w, r)
	}
	if z == nil {
		return dns.RcodeServerFailure, nil
	}
	if r.Opcode == dns.OpcodeNotify {
		if z.isNotify(state) {
			m := new(dns.Msg)
			m.SetReply(r)
			m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
			w.WriteMsg(m)

			if ok, _ := z.shouldTransfer(); ok {
				log.Printf("[INFO] Valid notify from %s for %s: initiating transfer", state.IP(), zone)
				z.TransferIn()
			}

			return dns.RcodeSuccess, nil
		}
		log.Printf("[INFO] Dropping notify from %s for %s", state.IP(), zone)
		return dns.RcodeSuccess, nil
	}

	if z.Expired != nil && *z.Expired {
		log.Printf("[ERROR] Zone %s is expired", zone)
		return dns.RcodeServerFailure, nil
	}

	if state.QType() == dns.TypeAXFR || state.QType() == dns.TypeIXFR {
		xfr := Xfr{z}
		return xfr.ServeDNS(ctx, w, r)
	}

	answer, ns, extra, result := z.Lookup(qname, state.QType(), state.Do())

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	switch result {
	case Success:
		m.Answer = answer
		m.Ns = ns
		m.Extra = extra
	case NameError:
		m.Ns = ns
		m.Rcode = dns.RcodeNameError
		fallthrough
	case NoData:
		m.Ns = ns
	case ServerFailure:
		return dns.RcodeServerFailure, nil
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
			log.Printf("[ERROR] Failed to parse %s: %v", origin, x.Error)
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
