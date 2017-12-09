// Package nsid implements NSID protocol
package nsid

import (
	"encoding/hex"

	"github.com/coredns/coredns/plugin"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Nsid plugin
type Nsid struct {
	Next plugin.Handler
	Data string
}

// ResponseWriter is a response writer that adds NSID response
type ResponseWriter struct {
	dns.ResponseWriter
	Data string
}

// ServeDNS implements the plugin.Handler interface.
func (n Nsid) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	if option := r.IsEdns0(); option != nil {
		for _, o := range option.Option {
			if _, ok := o.(*dns.EDNS0_NSID); ok {
				nw := &ResponseWriter{ResponseWriter: w, Data: n.Data}
				return plugin.NextOrFailure(n.Name(), n.Next, ctx, nw, r)
			}
		}
	}
	return plugin.NextOrFailure(n.Name(), n.Next, ctx, w, r)
}

// WriteMsg implements the dns.ResponseWriter interface.
func (w *ResponseWriter) WriteMsg(res *dns.Msg) error {
	if option := res.IsEdns0(); option != nil {
		for _, o := range option.Option {
			if e, ok := o.(*dns.EDNS0_NSID); ok {
				e.Code = dns.EDNS0NSID
				e.Nsid = hex.EncodeToString([]byte(w.Data))
			}
		}
	}
	returned := w.ResponseWriter.WriteMsg(res)
	return returned
}

// Name implements the Handler interface.
func (n Nsid) Name() string { return "nsid" }
