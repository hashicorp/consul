package cache

import (
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
)

// ServeDNS implements the middleware.Handler interface.
func (c *Cache) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	qname := state.Name()
	qtype := state.QType()
	zone := middleware.Zones(c.Zones).Matches(qname)
	if zone == "" {
		return c.Next.ServeDNS(ctx, w, r)
	}

	do := state.Do() // might need more from OPT record? Like the actual bufsize?

	if i, ok, expired := c.get(qname, qtype, do); ok && !expired {

		resp := i.toMsg(r)
		state.SizeAndDo(resp)
		w.WriteMsg(resp)

		cacheHitCount.WithLabelValues(zone).Inc()

		return dns.RcodeSuccess, nil
	}

	cacheMissCount.WithLabelValues(zone).Inc()

	crr := &ResponseWriter{w, c}
	return c.Next.ServeDNS(ctx, crr, r)
}

func (c *Cache) get(qname string, qtype uint16, do bool) (*item, bool, bool) {
	k := rawKey(qname, qtype, do)

	if i, ok := c.ncache.Get(k); ok {
		return i.(*item), ok, i.(*item).expired(time.Now())
	}

	if i, ok := c.pcache.Get(k); ok {
		return i.(*item), ok, i.(*item).expired(time.Now())
	}
	return nil, false, false
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
