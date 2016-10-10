// Package cache implements a cache.
package cache

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/response"

	"github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

// Cache is middleware that looks up responses in a cache and caches replies.
// It has a success and a denial of existence cache.
type Cache struct {
	Next  middleware.Handler
	Zones []string

	ncache *lru.Cache
	ncap   int
	nttl   time.Duration

	pcache *lru.Cache
	pcap   int
	pttl   time.Duration
}

// Return key under which we store the item.
func key(m *dns.Msg, t response.Type, do bool) string {
	if m.Truncated {
		// TODO(miek): wise to store truncated responses?
		return ""
	}
	if t == response.OtherError {
		return ""
	}

	qtype := m.Question[0].Qtype
	qname := strings.ToLower(m.Question[0].Name)
	return rawKey(qname, qtype, do)
}

func rawKey(qname string, qtype uint16, do bool) string {
	if do {
		return "1" + qname + "." + strconv.Itoa(int(qtype))
	}
	return "0" + qname + "." + strconv.Itoa(int(qtype))
}

// ResponseWriter is a response writer that caches the reply message.
type ResponseWriter struct {
	dns.ResponseWriter
	*Cache
}

// WriteMsg implements the dns.ResponseWriter interface.
func (c *ResponseWriter) WriteMsg(res *dns.Msg) error {
	do := false
	mt, opt := response.Typify(res)
	if opt != nil {
		do = opt.Do()
	}

	key := key(res, mt, do)

	duration := c.pttl
	if mt == response.NameError || mt == response.NoData {
		duration = c.nttl
	}

	msgTTL := minMsgTTL(res, mt)
	if msgTTL < duration {
		duration = msgTTL
	}

	if key != "" {
		c.set(res, key, mt, duration)
	}

	setMsgTTL(res, uint32(duration.Seconds()))

	return c.ResponseWriter.WriteMsg(res)
}

func (c *ResponseWriter) set(m *dns.Msg, key string, mt response.Type, duration time.Duration) {
	if key == "" {
		log.Printf("[ERROR] Caching called with empty cache key")
		return
	}

	switch mt {
	case response.NoError, response.Delegation:
		i := newItem(m, duration)
		c.pcache.Add(key, i)

	case response.NameError, response.NoData:
		i := newItem(m, duration)
		c.ncache.Add(key, i)

	case response.OtherError:
		// don't cache these
		// TODO(miek): what do we do with these?
	default:
		log.Printf("[WARNING] Caching called with unknown classification: %d", mt)
	}
}

// Write implements the dns.ResponseWriter interface.
func (c *ResponseWriter) Write(buf []byte) (int, error) {
	log.Printf("[WARNING] Caching called with Write: not caching reply")
	n, err := c.ResponseWriter.Write(buf)
	return n, err
}

const (
	maxTTL  = 1 * time.Hour
	maxNTTL = 30 * time.Minute
	minTTL  = 5 * time.Second

	defaultCap = 10000 // default capacity of the cache.
)
