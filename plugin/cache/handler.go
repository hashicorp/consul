package cache

import (
	"context"
	"math"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

// ServeDNS implements the plugin.Handler interface.
func (c *Cache) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	zone := plugin.Zones(c.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(c.Name(), c.Next, ctx, w, r)
	}

	now := c.now().UTC()

	server := metrics.WithServer(ctx)

	ttl := 0
	i := c.getIgnoreTTL(now, state, server)
	if i != nil {
		ttl = i.ttl(now)
	}
	if i == nil || -ttl >= int(c.staleUpTo.Seconds()) {
		crr := &ResponseWriter{ResponseWriter: w, Cache: c, state: state, server: server}
		return plugin.NextOrFailure(c.Name(), c.Next, ctx, crr, r)
	}
	if ttl < 0 {
		servedStale.WithLabelValues(server).Inc()
		// Adjust the time to get a 0 TTL in the reply built from a stale item.
		now = now.Add(time.Duration(ttl) * time.Second)
		go func() {
			r := r.Copy()
			crr := &ResponseWriter{Cache: c, state: state, server: server, prefetch: true, remoteAddr: w.LocalAddr()}
			plugin.NextOrFailure(c.Name(), c.Next, ctx, crr, r)
		}()
	}
	resp := i.toMsg(r, now)
	w.WriteMsg(resp)

	if c.shouldPrefetch(i, now) {
		go c.doPrefetch(ctx, state, server, i, now)
	}
	return dns.RcodeSuccess, nil
}

func (c *Cache) doPrefetch(ctx context.Context, state request.Request, server string, i *item, now time.Time) {
	cw := newPrefetchResponseWriter(server, state, c)

	cachePrefetches.WithLabelValues(server).Inc()
	plugin.NextOrFailure(c.Name(), c.Next, ctx, cw, state.Req)

	// When prefetching we loose the item i, and with it the frequency
	// that we've gathered sofar. See we copy the frequencies info back
	// into the new item that was stored in the cache.
	if i1 := c.exists(state); i1 != nil {
		i1.Freq.Reset(now, i.Freq.Hits())
	}
}

func (c *Cache) shouldPrefetch(i *item, now time.Time) bool {
	if c.prefetch <= 0 {
		return false
	}
	i.Freq.Update(c.duration, now)
	threshold := int(math.Ceil(float64(c.percentage) / 100 * float64(i.origTTL)))
	return i.Freq.Hits() >= c.prefetch && i.ttl(now) <= threshold
}

// Name implements the Handler interface.
func (c *Cache) Name() string { return "cache" }

func (c *Cache) get(now time.Time, state request.Request, server string) (*item, bool) {
	k := hash(state.Name(), state.QType(), state.Do())

	if i, ok := c.ncache.Get(k); ok && i.(*item).ttl(now) > 0 {
		cacheHits.WithLabelValues(server, Denial).Inc()
		return i.(*item), true
	}

	if i, ok := c.pcache.Get(k); ok && i.(*item).ttl(now) > 0 {
		cacheHits.WithLabelValues(server, Success).Inc()
		return i.(*item), true
	}
	cacheMisses.WithLabelValues(server).Inc()
	return nil, false
}

// getIgnoreTTL unconditionally returns an item if it exists in the cache.
func (c *Cache) getIgnoreTTL(now time.Time, state request.Request, server string) *item {
	k := hash(state.Name(), state.QType(), state.Do())

	if i, ok := c.ncache.Get(k); ok {
		ttl := i.(*item).ttl(now)
		if ttl > 0 || (c.staleUpTo > 0 && -ttl < int(c.staleUpTo.Seconds())) {
			cacheHits.WithLabelValues(server, Denial).Inc()
		}
		return i.(*item)
	}
	if i, ok := c.pcache.Get(k); ok {
		ttl := i.(*item).ttl(now)
		if ttl > 0 || (c.staleUpTo > 0 && -ttl < int(c.staleUpTo.Seconds())) {
			cacheHits.WithLabelValues(server, Success).Inc()
		}
		return i.(*item)
	}
	return nil
}

func (c *Cache) exists(state request.Request) *item {
	k := hash(state.Name(), state.QType(), state.Do())
	if i, ok := c.ncache.Get(k); ok {
		return i.(*item)
	}
	if i, ok := c.pcache.Get(k); ok {
		return i.(*item)
	}
	return nil
}

var (
	cacheSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "cache",
		Name:      "size",
		Help:      "The number of elements in the cache.",
	}, []string{"server", "type"})

	cacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "cache",
		Name:      "hits_total",
		Help:      "The count of cache hits.",
	}, []string{"server", "type"})

	cacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "cache",
		Name:      "misses_total",
		Help:      "The count of cache misses.",
	}, []string{"server"})

	cachePrefetches = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "cache",
		Name:      "prefetch_total",
		Help:      "The number of time the cache has prefetched a cached item.",
	}, []string{"server"})

	cacheDrops = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "cache",
		Name:      "drops_total",
		Help:      "The number responses that are not cached, because the reply is malformed.",
	}, []string{"server"})

	servedStale = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "cache",
		Name:      "served_stale_total",
		Help:      "The number of requests served from stale cache entries.",
	}, []string{"server"})
)
