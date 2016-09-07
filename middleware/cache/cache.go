package cache

import (
	"log"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/response"

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

func cacheKey(m *dns.Msg, t response.Type, do bool) string {
	if m.Truncated {
		return ""
	}

	qtype := m.Question[0].Qtype
	qname := strings.ToLower(m.Question[0].Name)
	switch t {
	case response.Success:
		fallthrough
	case response.Delegation:
		return successKey(qname, qtype, do)
	case response.NameError:
		return nameErrorKey(qname, do)
	case response.NoData:
		return noDataKey(qname, qtype, do)
	case response.OtherError:
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
	mt, opt := response.Classify(res)
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

func (c *CachingResponseWriter) set(m *dns.Msg, key string, mt response.Type) {
	if key == "" {
		log.Printf("[ERROR] Caching called with empty cache key")
		return
	}

	duration := c.cap
	switch mt {
	case response.Success, response.Delegation:
		if c.cap == 0 {
			duration = minTtl(m.Answer, mt)
		}
		i := newItem(m, duration)

		c.cache.Set(key, i, duration)
	case response.NameError, response.NoData:
		if c.cap == 0 {
			duration = minTtl(m.Ns, mt)
		}
		i := newItem(m, duration)

		c.cache.Set(key, i, duration)
	case response.OtherError:
		// don't cache these
	default:
		log.Printf("[WARNING] Caching called with unknown middleware MsgType: %d", mt)
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

func minTtl(rrs []dns.RR, mt response.Type) time.Duration {
	if mt != response.Success && mt != response.NameError && mt != response.NoData {
		return 0
	}

	minTtl := maxTtl
	for _, r := range rrs {
		switch mt {
		case response.NameError, response.NoData:
			if r.Header().Rrtype == dns.TypeSOA {
				return time.Duration(r.(*dns.SOA).Minttl) * time.Second
			}
		case response.Success, response.Delegation:
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
