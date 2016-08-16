package dnssec

import (
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/singleflight"

	"github.com/miekg/dns"
	gcache "github.com/patrickmn/go-cache"
)

type Dnssec struct {
	Next     middleware.Handler
	zones    []string
	keys     []*DNSKEY
	inflight *singleflight.Group
	cache    *gcache.Cache
}

func NewDnssec(zones []string, keys []*DNSKEY, next middleware.Handler) Dnssec {
	return Dnssec{Next: next,
		zones:    zones,
		keys:     keys,
		cache:    gcache.New(defaultDuration, purgeDuration),
		inflight: new(singleflight.Group),
	}
}

// Sign signs the message m. it takes care of negative or nodata responses. It
// uses NSEC black lies for authenticated denial of existence. Signatures
// creates will be cached for a short while. By default we sign for 8 days,
// starting 3 hours ago.
func (d Dnssec) Sign(state middleware.State, zone string, now time.Time) *dns.Msg {
	req := state.Req
	mt, _ := middleware.Classify(req) // TODO(miek): need opt record here?
	if mt == middleware.Delegation {
		return req
	}

	incep, expir := incepExpir(now)

	if mt == middleware.NameError {
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
			sig := k.NewRRSIG(signerName, ttl, incep, expir)
			e = sig.Sign(k.s, rrs)
			sigs[i] = sig
		}
		d.set(k, sigs)
		return sigs, e
	})
	return sigs.([]dns.RR), err
}

func (d Dnssec) set(key string, sigs []dns.RR) {
	// we insert the sigs with a duration that is 24 hours less then the expiration, as these
	// sigs have *just* been made the duration is 7 days.
	d.cache.Set(key, sigs, eightDays-24*time.Hour)
}

func (d Dnssec) get(key string) ([]dns.RR, bool) {
	if s, ok := d.cache.Get(key); ok {
		return s.([]dns.RR), true
	}
	return nil, false
}

func incepExpir(now time.Time) (uint32, uint32) {
	incep := uint32(now.Add(-3 * time.Hour).Unix()) // -(2+1) hours, be sure to catch daylight saving time and such
	expir := uint32(now.Add(eightDays).Unix())      // sign for 8 days
	return incep, expir
}

const (
	purgeDuration   = 3 * time.Hour
	defaultDuration = 24 * time.Hour
	eightDays       = 8 * 24 * time.Hour
)
