// Package auto implements an on-the-fly loading file backend.
package auto

import (
	"context"
	"regexp"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

type (
	// Auto holds the zones and the loader configuration for automatically loading zones.
	Auto struct {
		Next plugin.Handler
		*Zones

		metrics *metrics.Metrics
		loader
	}

	loader struct {
		directory string
		template  string
		re        *regexp.Regexp

		// In the future this should be something like ZoneMeta that contains all this stuff.
		transferTo     []string
		ReloadInterval time.Duration
		upstream       *upstream.Upstream // Upstream for looking up names during the resolution process.

		duration time.Duration
	}
)

// ServeDNS implements the plugin.Handler interface.
func (a Auto) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	// Precheck with the origins, i.e. are we allowed to look here?
	zone := plugin.Zones(a.Zones.Origins()).Matches(qname)
	if zone == "" {
		return plugin.NextOrFailure(a.Name(), a.Next, ctx, w, r)
	}

	// Now the real zone.
	zone = plugin.Zones(a.Zones.Names()).Matches(qname)

	a.Zones.RLock()
	z, ok := a.Zones.Z[zone]
	a.Zones.RUnlock()

	if !ok || z == nil {
		return dns.RcodeServerFailure, nil
	}

	if state.QType() == dns.TypeAXFR || state.QType() == dns.TypeIXFR {
		xfr := file.Xfr{Zone: z}
		return xfr.ServeDNS(ctx, w, r)
	}

	answer, ns, extra, result := z.Lookup(ctx, state, qname)

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer, m.Ns, m.Extra = answer, ns, extra

	switch result {
	case file.Success:
	case file.NoData:
	case file.NameError:
		m.Rcode = dns.RcodeNameError
	case file.Delegation:
		m.Authoritative = false
	case file.ServerFailure:
		return dns.RcodeServerFailure, nil
	}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (a Auto) Name() string { return "auto" }
