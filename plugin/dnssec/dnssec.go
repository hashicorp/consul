// Package dnssec implements a plugin that signs responses on-the-fly using
// NSEC black lies.
package dnssec

import (
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/cache"
	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/plugin/pkg/singleflight"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Dnssec signs the reply on-the-fly.
type Dnssec struct {
	Next plugin.Handler

	zones    []string
	keys     []*DNSKEY
	inflight *singleflight.Group
	cache    *cache.Cache
}

// New returns a new Dnssec.
func New(zones []string, keys []*DNSKEY, next plugin.Handler, c *cache.Cache) Dnssec {
	return Dnssec{Next: next,
		zones:    zones,
		keys:     keys,
		cache:    c,
		inflight: new(singleflight.Group),
	}
}

// Sign signs the message in state. it takes care of negative or nodata responses. It
// uses NSEC black lies for authenticated denial of existence. For delegations it
// will insert DS records and sign those.
// Signatures will be cached for a short while. By default we sign for 8 days,
// starting 3 hours ago.
func (d Dnssec) Sign(state request.Request, now time.Time) *dns.Msg {
	req := state.Req

	incep, expir := incepExpir(now)

	mt, _ := response.Typify(req, time.Now().UTC()) // TODO(miek): need opt record here?
	if mt == response.Delegation {
		// This reverts 11203e44. Reverting with git revert leads to conflicts in dnskey.go, and I'm
		// not sure yet if we just should fiddle with inserting DSs or not.
		// Easy way to, see #1211 for discussion.
		/*
			ttl := req.Ns[0].Header().Ttl

			ds := []dns.RR{}
			for i := range d.keys {
				ds = append(ds, d.keys[i].D)
			}
			if sigs, err := d.sign(ds, zone, ttl, incep, expir); err == nil {
				req.Ns = append(req.Ns, ds...)
				req.Ns = append(req.Ns, sigs...)
			}
		*/
		return req
	}

	if mt == response.NameError || mt == response.NoData {
		if req.Ns[0].Header().Rrtype != dns.TypeSOA || len(req.Ns) > 1 {
			return req
		}

		ttl := req.Ns[0].Header().Ttl

		if sigs, err := d.sign(req.Ns, state.Zone, ttl, incep, expir); err == nil {
			req.Ns = append(req.Ns, sigs...)
		}
		if sigs, err := d.nsec(state, mt, ttl, incep, expir); err == nil {
			req.Ns = append(req.Ns, sigs...)
		}
		if len(req.Ns) > 1 { // actually added nsec and sigs, reset the rcode
			req.Rcode = dns.RcodeSuccess
		}
		return req
	}

	for _, r := range rrSets(req.Answer) {
		ttl := r[0].Header().Ttl
		if sigs, err := d.sign(r, state.Zone, ttl, incep, expir); err == nil {
			req.Answer = append(req.Answer, sigs...)
		}
	}
	for _, r := range rrSets(req.Ns) {
		ttl := r[0].Header().Ttl
		if sigs, err := d.sign(r, state.Zone, ttl, incep, expir); err == nil {
			req.Ns = append(req.Ns, sigs...)
		}
	}
	for _, r := range rrSets(req.Extra) {
		ttl := r[0].Header().Ttl
		if sigs, err := d.sign(r, state.Zone, ttl, incep, expir); err == nil {
			req.Extra = append(sigs, req.Extra...) // prepend to leave OPT alone
		}
	}
	return req
}

func (d Dnssec) sign(rrs []dns.RR, signerName string, ttl, incep, expir uint32) ([]dns.RR, error) {
	k := hash(rrs)
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

func (d Dnssec) set(key uint32, sigs []dns.RR) {
	d.cache.Add(key, sigs)
}

func (d Dnssec) get(key uint32) ([]dns.RR, bool) {
	if s, ok := d.cache.Get(key); ok {
		// we sign for 8 days, check if a signature in the cache reached 3/4 of that
		is75 := time.Now().UTC().Add(sixDays)
		for _, rr := range s.([]dns.RR) {
			if !rr.(*dns.RRSIG).ValidityPeriod(is75) {
				cacheMisses.Inc()
				return nil, false
			}
		}

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
	sixDays    = 6 * 24 * time.Hour
	defaultCap = 10000 // default capacity of the cache.
)
