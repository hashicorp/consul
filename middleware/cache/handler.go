package cache

import (
	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (c Cache) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}

	qname := state.Name()
	qtype := state.QType()
	zone := middleware.Zones(c.Zones).Matches(qname)
	if zone == "" {
		return c.Next.ServeDNS(ctx, w, r)
	}

	do := state.Do() // might need more from OPT record?

	if i, ok := c.Get(qname, qtype, do); ok {
		resp := i.toMsg(r)
		state.SizeAndDo(resp)
		w.WriteMsg(resp)

		cacheHitCount.WithLabelValues(zone).Inc()
		return dns.RcodeSuccess, nil
	}
	cacheMissCount.WithLabelValues(zone).Inc()

	crr := NewCachingResponseWriter(w, c.cache, c.cap)
	return c.Next.ServeDNS(ctx, crr, r)
}

func (c Cache) Get(qname string, qtype uint16, do bool) (*item, bool) {
	nxdomain := nameErrorKey(qname, do)
	if i, ok := c.cache.Get(nxdomain); ok {
		return i.(*item), true
	}

	successOrNoData := successKey(qname, qtype, do)
	if i, ok := c.cache.Get(successOrNoData); ok {
		return i.(*item), true
	}
	return nil, false
}

var (
	cacheHitCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "hit_count_total",
		Help:      "Counter of DNS requests that were found in the cache.",
	}, []string{"zone"})

	cacheMissCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "miss_count_total",
		Help:      "Counter of DNS requests that were not found in the cache.",
	}, []string{"zone"})
)

const subsystem = "cache"

func init() {
	prometheus.MustRegister(cacheHitCount)
	prometheus.MustRegister(cacheMissCount)
}
