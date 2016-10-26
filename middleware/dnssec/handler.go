package dnssec

import (
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (d Dnssec) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

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

	drr := &ResponseWriter{w, d}
	return d.Next.ServeDNS(ctx, drr, r)
}

var (
	cacheSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "size_guage",
		Help:      "Gauge of number of elements in the cache.",
	}, []string{"type"})

	cacheCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "capacity_gauge",
		Help:      "Gauge of cache's capacity.",
	}, []string{"type"})
)

func (d Dnssec) Name() string { return "dnssec" }

const subsystem = "dnssec"

func init() {
	prometheus.MustRegister(cacheSize)
	prometheus.MustRegister(cacheCapacity)
}
