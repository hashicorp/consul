package dnssec

import (
	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (d Dnssec) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}

	do := state.Do()
	qname := state.Name()
	qtype := state.QType()
	zone := middleware.Zones(d.zones).Matches(qname)
	if zone == "" {
		return d.Next.ServeDNS(ctx, w, r)
	}

	// Intercept queries for DNSKEY, but only if one of the zones matches the qname, otherwise we let
	// the query through.
	if qtype == dns.TypeDNSKEY {
		for _, z := range d.zones {
			if qname == z {
				resp := d.getDNSKEY(state, z, do)
				resp.Authoritative = true
				state.SizeAndDo(resp)
				w.WriteMsg(resp)
				return dns.RcodeSuccess, nil
			}
		}
	}

	drr := NewDnssecResponseWriter(w, d)
	return d.Next.ServeDNS(ctx, drr, r)
}

var (
	cacheHitCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "hit_count_total",
		Help:      "Counter of signatures that were found in the cache.",
	}, []string{"zone"})

	cacheMissCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "miss_count_total",
		Help:      "Counter of signatures that were not found in the cache.",
	}, []string{"zone"})
)

const subsystem = "dnssec"

func init() {
	prometheus.MustRegister(cacheHitCount)
	prometheus.MustRegister(cacheMissCount)
}
