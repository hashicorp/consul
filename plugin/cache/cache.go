// Package cache implements a cache.
package cache

import (
	"encoding/binary"
	"hash/fnv"
	"log"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/cache"
	"github.com/coredns/coredns/plugin/pkg/response"

	"github.com/miekg/dns"
)

// Cache is plugin that looks up responses in a cache and caches replies.
// It has a success and a denial of existence cache.
type Cache struct {
	Next  plugin.Handler
	Zones []string

	ncache *cache.Cache
	ncap   int
	nttl   time.Duration

	pcache *cache.Cache
	pcap   int
	pttl   time.Duration

	// Prefetch.
	prefetch   int
	duration   time.Duration
	percentage int

	// Testing.
	now func() time.Time
}

// New returns an initialized Cache with default settings. It's up to the
// caller to set the Next handler.
func New() *Cache {
	return &Cache{
		Zones:      []string{"."},
		pcap:       defaultCap,
		pcache:     cache.New(defaultCap),
		pttl:       maxTTL,
		ncap:       defaultCap,
		ncache:     cache.New(defaultCap),
		nttl:       maxNTTL,
		prefetch:   0,
		duration:   1 * time.Minute,
		percentage: 10,
		now:        time.Now,
	}
}

// Return key under which we store the item, -1 will be returned if we don't store the
// message.
// Currently we do not cache Truncated, errors zone transfers or dynamic update messages.
func key(m *dns.Msg, t response.Type, do bool) int {
	// We don't store truncated responses.
	if m.Truncated {
		return -1
	}
	// Nor errors or Meta or Update
	if t == response.OtherError || t == response.Meta || t == response.Update {
		return -1
	}

	return int(hash(m.Question[0].Name, m.Question[0].Qtype, do))
}

var one = []byte("1")
var zero = []byte("0")

func hash(qname string, qtype uint16, do bool) uint32 {
	h := fnv.New32()

	if do {
		h.Write(one)
	} else {
		h.Write(zero)
	}

	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, qtype)
	h.Write(b)

	for i := range qname {
		c := qname[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		h.Write([]byte{c})
	}

	return h.Sum32()
}

// ResponseWriter is a response writer that caches the reply message.
type ResponseWriter struct {
	dns.ResponseWriter
	*Cache

	prefetch bool // When true write nothing back to the client.
}

// WriteMsg implements the dns.ResponseWriter interface.
func (w *ResponseWriter) WriteMsg(res *dns.Msg) error {
	do := false
	mt, opt := response.Typify(res, w.now().UTC())
	if opt != nil {
		do = opt.Do()
	}

	// key returns empty string for anything we don't want to cache.
	key := key(res, mt, do)

	duration := w.pttl
	if mt == response.NameError || mt == response.NoData {
		duration = w.nttl
	}

	msgTTL := minMsgTTL(res, mt)
	if msgTTL < duration {
		duration = msgTTL
	}

	if key != -1 && duration > 0 {
		w.set(res, key, mt, duration)

		cacheSize.WithLabelValues(Success).Set(float64(w.pcache.Len()))
		cacheSize.WithLabelValues(Denial).Set(float64(w.ncache.Len()))
	}

	if w.prefetch {
		return nil
	}

	// Apply capped TTL to this reply to avoid jarring TTL experience 1799 -> 8 (e.g.)
	ttl := uint32(duration.Seconds())
	for i := range res.Answer {
		res.Answer[i].Header().Ttl = ttl
	}
	for i := range res.Ns {
		res.Ns[i].Header().Ttl = ttl
	}
	for i := range res.Extra {
		if res.Extra[i].Header().Rrtype != dns.TypeOPT {
			res.Extra[i].Header().Ttl = ttl
		}
	}
	return w.ResponseWriter.WriteMsg(res)
}

func (w *ResponseWriter) set(m *dns.Msg, key int, mt response.Type, duration time.Duration) {
	if key == -1 || duration == 0 {
		return
	}

	switch mt {
	case response.NoError, response.Delegation:
		i := newItem(m, w.now(), duration)
		w.pcache.Add(uint32(key), i)

	case response.NameError, response.NoData:
		i := newItem(m, w.now(), duration)
		w.ncache.Add(uint32(key), i)

	case response.OtherError:
		// don't cache these
	default:
		log.Printf("[WARNING] Caching called with unknown classification: %d", mt)
	}
}

// Write implements the dns.ResponseWriter interface.
func (w *ResponseWriter) Write(buf []byte) (int, error) {
	log.Printf("[WARNING] Caching called with Write: not caching reply")
	if w.prefetch {
		return 0, nil
	}
	n, err := w.ResponseWriter.Write(buf)
	return n, err
}

const (
	maxTTL  = 1 * time.Hour
	maxNTTL = 30 * time.Minute

	defaultCap = 10000 // default capacity of the cache.

	// Success is the class for caching positive caching.
	Success = "success"
	// Denial is the class defined for negative caching.
	Denial = "denial"
)
