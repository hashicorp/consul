// Package errors implements an HTTP error handling plugin.
package errors

import (
	"context"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// errorHandler handles DNS errors (and errors from other plugin).
type errorHandler struct{ Next plugin.Handler }

// ServeDNS implements the plugin.Handler interface.
func (h errorHandler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	rcode, err := plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)

	if err != nil {
		state := request.Request{W: w, Req: r}
		clog.Errorf("%d %s %s: %v", rcode, state.Name(), state.Type(), err)
	}

	return rcode, err
}

func (h errorHandler) Name() string { return "errors" }
