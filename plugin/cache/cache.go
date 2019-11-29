// Package cache implements a cache.
package cache

import (
	"hash/fnv"
	"net"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/cache"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Cache is a plugin that looks up responses in a cache and caches replies.
// It has a success and a denial of existence cache.
type Cache struct {
	Next  plugin.Handler
	Zones []string

	ncache  *cache.Cache
	ncap    int
	nttl    time.Duration
	minnttl time.Duration

	pcache  *cache.Cache
	pcap    int
	pttl    time.Duration
	minpttl time.Duration

	// Prefetch.
	prefetch   int
	duration   time.Duration
	percentage int

	staleUpTo time.Duration

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
		minpttl:    minTTL,
		ncap:       defaultCap,
		ncache:     cache.New(defaultCap),
		nttl:       maxNTTL,
		minnttl:    minNTTL,
		prefetch:   0,
		duration:   1 * time.Minute,
		percentage: 10,
		now:        time.Now,
	}
}

// key returns key under which we store the item, -1 will be returned if we don't store the message.
// Currently we do not cache Truncated, errors zone transfers or dynamic update messages.
// qname holds the already lowercased qname.
func key(qname string, m *dns.Msg, t response.Type, do bool) (bool, uint64) {
	// We don't store truncated responses.
	if m.Truncated {
		return false, 0
	}
	// Nor errors or Meta or Update
	if t == response.OtherError || t == response.Meta || t == response.Update {
		return false, 0
	}

	return true, hash(qname, m.Question[0].Qtype, do)
}

var one = []byte("1")
var zero = []byte("0")

func hash(qname string, qtype uint16, do bool) uint64 {
	h := fnv.New64()

	if do {
		h.Write(one)
	} else {
		h.Write(zero)
	}

	h.Write([]byte{byte(qtype >> 8)})
	h.Write([]byte{byte(qtype)})
	h.Write([]byte(qname))
	return h.Sum64()
}

func computeTTL(msgTTL, minTTL, maxTTL time.Duration) time.Duration {
	ttl := msgTTL
	if ttl < minTTL {
		ttl = minTTL
	}
	if ttl > maxTTL {
		ttl = maxTTL
	}
	return ttl
}

// ResponseWriter is a response writer that caches the reply message.
type ResponseWriter struct {
	dns.ResponseWriter
	*Cache
	state  request.Request
	server string // Server handling the request.

	prefetch   bool // When true write nothing back to the client.
	remoteAddr net.Addr
}

// newPrefetchResponseWriter returns a Cache ResponseWriter to be used in
// prefetch requests. It ensures RemoteAddr() can be called even after the
// original connection has already been closed.
func newPrefetchResponseWriter(server string, state request.Request, c *Cache) *ResponseWriter {
	// Resolve the address now, the connection might be already closed when the
	// actual prefetch request is made.
	addr := state.W.RemoteAddr()
	// The protocol of the client triggering a cache prefetch doesn't matter.
	// The address type is used by request.Proto to determine the response size,
	// and using TCP ensures the message isn't unnecessarily truncated.
	if u, ok := addr.(*net.UDPAddr); ok {
		addr = &net.TCPAddr{IP: u.IP, Port: u.Port, Zone: u.Zone}
	}

	return &ResponseWriter{
		ResponseWriter: state.W,
		Cache:          c,
		state:          state,
		server:         server,
		prefetch:       true,
		remoteAddr:     addr,
	}
}

// RemoteAddr implements the dns.ResponseWriter interface.
func (w *ResponseWriter) RemoteAddr() net.Addr {
	if w.remoteAddr != nil {
		return w.remoteAddr
	}
	return w.ResponseWriter.RemoteAddr()
}

// WriteMsg implements the dns.ResponseWriter interface.
func (w *ResponseWriter) WriteMsg(res *dns.Msg) error {
	do := false
	mt, opt := response.Typify(res, w.now().UTC())
	if opt != nil {
		do = opt.Do()
	}

	// key returns empty string for anything we don't want to cache.
	hasKey, key := key(w.state.Name(), res, mt, do)

	msgTTL := dnsutil.MinimalTTL(res, mt)
	var duration time.Duration
	if mt == response.NameError || mt == response.NoData {
		duration = computeTTL(msgTTL, w.minnttl, w.nttl)
	} else if mt == response.ServerError {
		// use default ttl which is 5s
		duration = minTTL
	} else {
		duration = computeTTL(msgTTL, w.minpttl, w.pttl)
	}

	if hasKey && duration > 0 {
		if w.state.Match(res) {
			w.set(res, key, mt, duration)
			cacheSize.WithLabelValues(w.server, Success).Set(float64(w.pcache.Len()))
			cacheSize.WithLabelValues(w.server, Denial).Set(float64(w.ncache.Len()))
		} else {
			// Don't log it, but increment counter
			cacheDrops.WithLabelValues(w.server).Inc()
		}
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

func (w *ResponseWriter) set(m *dns.Msg, key uint64, mt response.Type, duration time.Duration) {
	// duration is expected > 0
	// and key is valid
	switch mt {
	case response.NoError, response.Delegation:
		i := newItem(m, w.now(), duration)
		w.pcache.Add(key, i)

	case response.NameError, response.NoData, response.ServerError:
		i := newItem(m, w.now(), duration)
		w.ncache.Add(key, i)

	case response.OtherError:
		// don't cache these
	default:
		log.Warningf("Caching called with unknown classification: %d", mt)
	}
}

// Write implements the dns.ResponseWriter interface.
func (w *ResponseWriter) Write(buf []byte) (int, error) {
	log.Warning("Caching called with Write: not caching reply")
	if w.prefetch {
		return 0, nil
	}
	n, err := w.ResponseWriter.Write(buf)
	return n, err
}

const (
	maxTTL  = dnsutil.MaximumDefaulTTL
	minTTL  = dnsutil.MinimalDefaultTTL
	maxNTTL = dnsutil.MaximumDefaulTTL / 2
	minNTTL = dnsutil.MinimalDefaultTTL

	defaultCap = 10000 // default capacity of the cache.

	// Success is the class for caching positive caching.
	Success = "success"
	// Denial is the class defined for negative caching.
	Denial = "denial"
)
