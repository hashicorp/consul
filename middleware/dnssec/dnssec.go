// Package dnssec implements a middleware that signs responses on-the-fly using
// NSEC black lies.
package dnssec

import (
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/response"
	"github.com/miekg/coredns/middleware/pkg/singleflight"
	"github.com/miekg/coredns/request"

	"github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

// Dnssec signs the reply on-the-fly.
type Dnssec struct {
	Next middleware.Handler

	zones    []string
	keys     []*DNSKEY
	inflight *singleflight.Group
	cache    *lru.Cache
}

// New returns a new Dnssec.
func New(zones []string, keys []*DNSKEY, next middleware.Handler, cache *lru.Cache) Dnssec {
	return Dnssec{Next: next,
		zones:    zones,
		keys:     keys,
		cache:    cache,
		inflight: new(singleflight.Group),
	}
}

// Sign signs the message in state. it takes care of negative or nodata responses. It
// uses NSEC black lies for authenticated denial of existence. Signatures
// creates will be cached for a short while. By default we sign for 8 days,
// starting 3 hours ago.
func (d Dnssec) Sign(state request.Request, zone string, now time.Time) *dns.Msg {
	req := state.Req

	mt, _ := response.Typify(req) // TODO(miek): need opt record here?
	if mt == response.Delegation {
		return req
	}

	incep, expir := incepExpir(now)

	if mt == response.NameError {
		if req.Ns[0].Header().Rrtype != dns.TypeSOA || len(req.Ns) > 1 {
			return req
		}

		ttl := req.Ns[0].Header().Ttl

		if sigs, err := d.sign(req.Ns, zone, ttl, incep, expir); err == nil {
			req.Ns = append(req.Ns, sigs...)
		}
		if sigs, err := d.nsec(state.Name(), zone, ttl, incep, expir); err == nil {
			req.Ns = append(req.Ns, sigs...)
		}
		if len(req.Ns) > 1 { // actually added nsec and sigs, reset the rcode
			req.Rcode = dns.RcodeSuccess
		}
		return req
	}

	for _, r := range rrSets(req.Answer) {
		ttl := r[0].Header().Ttl
		if sigs, err := d.sign(r, zone, ttl, incep, expir); err == nil {
			req.Answer = append(req.Answer, sigs...)
		}
	}
	for _, r := range rrSets(req.Ns) {
		ttl := r[0].Header().Ttl
		if sigs, err := d.sign(r, zone, ttl, incep, expir); err == nil {
			req.Ns = append(req.Ns, sigs...)
		}
	}
	for _, r := range rrSets(req.Extra) {
		ttl := r[0].Header().Ttl
		if sigs, err := d.sign(r, zone, ttl, incep, expir); err == nil {
			req.Extra = append(sigs, req.Extra...) // prepend to leave OPT alone
		}
	}
	return req
}

func (d Dnssec) sign(rrs []dns.RR, signerName string, ttl, incep, expir uint32) ([]dns.RR, error) {
	k := key(rrs)
	sgs, ok := d.get(k)
	if ok {
		return sgs, nil
	}

	sigs, err := d.inflight.Do(k, func() (interface{}, error) {
		sigs := make([]dns.RR, len(d.keys))
		var e error
		for i, k := range d.keys {
			sig := k.newRRSIG(signerName, ttl, incep, expir)
			e = sig.Sign(k.s, rrs)
			sigs[i] = sig
		}
		d.set(k, sigs)
		return sigs, e
	})
	return sigs.([]dns.RR), err
}

func (d Dnssec) set(key string, sigs []dns.RR) {
	d.cache.Add(key, sigs)
}

func (d Dnssec) get(key string) ([]dns.RR, bool) {
	if s, ok := d.cache.Get(key); ok {
		cacheHits.Inc()
		return s.([]dns.RR), true
	}
	cacheMisses.Inc()
	return nil, false
}

func incepExpir(now time.Time) (uint32, uint32) {
	incep := uint32(now.Add(-3 * time.Hour).Unix()) // -(2+1) hours, be sure to catch daylight saving time and such
	expir := uint32(now.Add(eightDays).Unix())      // sign for 8 days
	return incep, expir
}

const (
	eightDays  = 8 * 24 * time.Hour
	defaultCap = 10000 // default capacity of the cache.
)
