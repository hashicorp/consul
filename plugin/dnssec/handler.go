package dnssec

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
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
	server := metrics.WithServer(ctx)

	// Intercept queries for DNSKEY, but only if one of the zones matches the qname, otherwise we let
	// the query through.
	if qtype == dns.TypeDNSKEY {
		for _, z := range d.zones {
			if qname == z {
				resp := d.getDNSKEY(state, z, do, server)
				resp.Authoritative = true
				w.WriteMsg(resp)
				return dns.RcodeSuccess, nil
			}
		}
	}

	if do {
		drr := &ResponseWriter{w, d, server}
		return plugin.NextOrFailure(d.Name(), d.Next, ctx, drr, r)
	}

	return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
}

var (
	cacheSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_size",
		Help:      "The number of elements in the dnssec cache.",
	}, []string{"server", "type"})

	cacheCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_capacity",
		Help:      "The dnssec cache's capacity.",
	}, []string{"server", "type"})

	cacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_hits_total",
		Help:      "The count of cache hits.",
	}, []string{"server"})

	cacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "dnssec",
		Name:      "cache_misses_total",
		Help:      "The count of cache misses.",
	}, []string{"server"})
)

// Name implements the Handler interface.
func (d Dnssec) Name() string { return "dnssec" }
