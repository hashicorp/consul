package dnssec

import (
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
)

// ServeDNS implements the plugin.Handler interface.
func (d Dnssec) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	do := state.Do()
	qname := state.Name()
	qtype := state.QType()
	zone := plugin.Zones(d.zones).Matches(qname)
	if zone == "" {
		return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
	}

	state.Zone = zone

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
	return plugin.NextOrFailure(d.Name(), d.Next, ctx, drr, r)
}

var (
	cacheSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_size",
		Help:      "The number of elements in the dnssec cache.",
	}, []string{"type"})

	cacheCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_capacity",
		Help:      "The dnssec cache's capacity.",
	}, []string{"type"})

	cacheHits = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_hits_total",
		Help:      "The count of cache hits.",
	})

	cacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_misses_total",
		Help:      "The count of cache misses.",
	})
)

// Name implements the Handler interface.
func (d Dnssec) Name() string { return "dnssec" }

var once sync.Once
