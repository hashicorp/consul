// Package forward implements a forwarding proxy. It caches an upstream net.Conn for some time, so if the same
// client returns the upstream's Conn will be precached. Depending on how you benchmark this looks to be
// 50% faster than just openening a new connection for every client. It works with UDP and TCP and uses
// inband healthchecking.
package forward

import (
	"crypto/tls"
	"log"
	"time"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Forward forward the request in state as-is. Unlike Lookup that adds EDNS0 suffix to the message.
// Forward may be called with a nil f, an error is returned in that case.
func (f *Forward) Forward(state request.Request) (*dns.Msg, error) {
	if f == nil {
		return nil, errNoForward
	}

	fails := 0
	for _, proxy := range f.list() {
		if proxy.Down(f.maxfails) {
			fails++
			if fails < len(f.proxies) {
				continue
			}
			// All upstream proxies are dead, assume healtcheck is complete broken and randomly
			// select an upstream to connect to.
			proxy = f.list()[0]
			log.Printf("[WARNING] All upstreams down, picking random one to connect to %s", proxy.host.addr)
		}

		ret, err := proxy.connect(context.Background(), state, f.forceTCP, true)
		if err != nil {
			log.Printf("[WARNING] Failed to connect to %s: %s", proxy.host.addr, err)
			if fails < len(f.proxies) {
				continue
			}
			break

		}

		return ret, nil
	}
	return nil, errNoHealthy
}

// Lookup will use name and type to forge a new message and will send that upstream. It will
// set any EDNS0 options correctly so that downstream will be able to process the reply.
// Lookup may be called with a nil f, an error is returned in that case.
func (f *Forward) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	if f == nil {
		return nil, errNoForward
	}

	req := new(dns.Msg)
	req.SetQuestion(name, typ)
	state.SizeAndDo(req)

	state2 := request.Request{W: state.W, Req: req}

	return f.Forward(state2)
}

// NewLookup returns a Forward that can be used for plugin that need an upstream to resolve external names.
func NewLookup(addr []string) *Forward {
	f := &Forward{maxfails: 2, tlsConfig: new(tls.Config), expire: defaultExpire, hcInterval: 2 * time.Second}
	for i := range addr {
		p := NewProxy(addr[i])
		f.SetProxy(p)
	}
	return f
}
