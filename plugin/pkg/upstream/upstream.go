// Package upstream abstracts a upstream lookups so that plugins
// can handle them in an unified way.
package upstream

import (
	"github.com/miekg/dns"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/request"
)

// Upstream is used to resolve CNAME targets
type Upstream struct {
	self    bool
	Forward *proxy.Proxy
}

// New creates a new Upstream for given destination(s). If dests is empty it default to upstreaming to
// the coredns process.
func New(dests []string) (Upstream, error) {
	u := Upstream{}
	if len(dests) == 0 {
		u.self = true
		return u, nil
	}
	u.self = false
	ups, err := dnsutil.ParseHostPortOrFile(dests...)
	if err != nil {
		return u, err
	}
	p := proxy.NewLookup(ups)
	u.Forward = &p
	return u, nil
}

// Lookup routes lookups to our selves or forward to a remote.
func (u Upstream) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	if u.self {
		req := new(dns.Msg)
		req.SetQuestion(name, typ)

		nw := nonwriter.New(state.W)
		server := state.Context.Value(dnsserver.Key{}).(*dnsserver.Server)

		server.ServeDNS(state.Context, nw, req)

		return nw.Msg, nil
	}

	if u.Forward != nil {
		return u.Forward.Lookup(state, name, typ)
	}

	return nil, nil
}
