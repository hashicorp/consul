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

		return dns.RcodeSuccess, nil
	}

	crr := &ResponseWriter{w, c}
	return c.Next.ServeDNS(ctx, crr, r)
}

// Name implements the Handler interface.
func (c *Cache) Name() string { return "cache" }

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

const subsystem = "cache"

func init() {
	prometheus.MustRegister(cacheSize)
	prometheus.MustRegister(cacheCapacity)
}
