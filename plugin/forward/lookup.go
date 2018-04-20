// Package forward implements a forwarding proxy. It caches an upstream net.Conn for some time, so if the same
// client returns the upstream's Conn will be precached. Depending on how you benchmark this looks to be
// 50% faster than just openening a new connection for every client. It works with UDP and TCP and uses
// inband healthchecking.
package forward

import (
	"github.com/coredns/coredns/request"

	"context"

	"github.com/miekg/dns"
)

// Forward forward the request in state as-is. Unlike Lookup that adds EDNS0 suffix to the message.
// Forward may be called with a nil f, an error is returned in that case.
func (f *Forward) Forward(state request.Request) (*dns.Msg, error) {
	if f == nil {
		return nil, errNoForward
	}

	fails := 0
	var upstreamErr error
	for _, proxy := range f.list() {
		if proxy.Down(f.maxfails) {
			fails++
			if fails < len(f.proxies) {
				continue
			}
			// All upstream proxies are dead, assume healtcheck is complete broken and randomly
			// select an upstream to connect to.
			proxy = f.list()[0]
		}

		ret, err := proxy.connect(context.Background(), state, f.forceTCP, true)

		ret, err = truncated(state, ret, err)
		upstreamErr = err

		if err != nil {
			if fails < len(f.proxies) {
				continue
			}
			break
		}

		// Check if the reply is correct; if not return FormErr.
		if !state.Match(ret) {
			return state.ErrorMessage(dns.RcodeFormatError), nil
		}

		return ret, err
	}

	if upstreamErr != nil {
		return nil, upstreamErr
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
// Note that the caller must run Close on the forward to stop the health checking goroutines.
func NewLookup(addr []string) *Forward {
	f := New()
	for i := range addr {
		p := NewProxy(addr[i], nil)
		f.SetProxy(p)
	}
	return f
}
