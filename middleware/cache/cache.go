package cache

import (
	"log"
	"time"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	gcache "github.com/patrickmn/go-cache"
)

// Cache is middleware that looks up responses in a cache and caches replies.
type Cache struct {
	Next  middleware.Handler
	Zones []string
	cache *gcache.Cache
	cap   time.Duration
}

func NewCache(ttl int, zones []string, next middleware.Handler) Cache {
	return Cache{Next: next, Zones: zones, cache: gcache.New(defaultDuration, purgeDuration), cap: time.Duration(ttl) * time.Second}
}

func cacheKey(m *dns.Msg, t middleware.MsgType, do bool) string {
	if m.Truncated {
		return ""
	}

	qtype := m.Question[0].Qtype
	qname := middleware.Name(m.Question[0].Name).Normalize()
	switch t {
	case middleware.Success:
		fallthrough
	case middleware.Delegation:
		return successKey(qname, qtype, do)
	case middleware.NameError:
		return nameErrorKey(qname, do)
	case middleware.NoData:
		return noDataKey(qname, qtype, do)
	case middleware.OtherError:
		return ""
	}
	return ""
}

type CachingResponseWriter struct {
	dns.ResponseWriter
	cache *gcache.Cache
	cap   time.Duration
}

func NewCachingResponseWriter(w dns.ResponseWriter, cache *gcache.Cache, cap time.Duration) *CachingResponseWriter {
	return &CachingResponseWriter{w, cache, cap}
}

func (c *CachingResponseWriter) WriteMsg(res *dns.Msg) error {
	do := false
	mt, opt := middleware.Classify(res)
	if opt != nil {
		do = opt.Do()
	}

	key := cacheKey(res, mt, do)
	c.set(res, key, mt)

	if c.cap != 0 {
		setCap(res, uint32(c.cap.Seconds()))
	}

	return c.ResponseWriter.WriteMsg(res)
}

func (c *CachingResponseWriter) set(m *dns.Msg, key string, mt middleware.MsgType) {
	if key == "" {
		// logger the log? TODO(miek)
		return
	}

	duration := c.cap
	switch mt {
	case middleware.Success, middleware.Delegation:
		if c.cap == 0 {
			duration = minTtl(m.Answer, mt)
		}
		i := newItem(m, duration)

		c.cache.Set(key, i, duration)
	case middleware.NameError, middleware.NoData:
		if c.cap == 0 {
			duration = minTtl(m.Ns, mt)
		}
		i := newItem(m, duration)

		c.cache.Set(key, i, duration)
	}
}

func (c *CachingResponseWriter) Write(buf []byte) (int, error) {
	log.Printf("[WARNING] Caching called with Write: not caching reply")
	n, err := c.ResponseWriter.Write(buf)
	return n, err
}

func (c *CachingResponseWriter) Hijack() {
	c.ResponseWriter.Hijack()
	return
}

func minTtl(rrs []dns.RR, mt middleware.MsgType) time.Duration {
	if mt != middleware.Success && mt != middleware.NameError && mt != middleware.NoData {
		return 0
	}

	minTtl := maxTtl
	for _, r := range rrs {
		switch mt {
		case middleware.NameError, middleware.NoData:
			if r.Header().Rrtype == dns.TypeSOA {
				return time.Duration(r.(*dns.SOA).Minttl) * time.Second
			}
		case middleware.Success, middleware.Delegation:
			if r.Header().Ttl < minTtl {
				minTtl = r.Header().Ttl
			}
		}
	}
	return time.Duration(minTtl) * time.Second
}

const (
	purgeDuration          = 1 * time.Minute
	defaultDuration        = 20 * time.Minute
	baseTtl                = 5 // minimum ttl that we will allow
	maxTtl          uint32 = 2 * 3600
)
