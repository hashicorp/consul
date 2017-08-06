package dnstap

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/dnstap/msg"
	"github.com/coredns/coredns/middleware/dnstap/taprw"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

type Dnstap struct {
	Next middleware.Handler
	Out  io.Writer
	Pack bool
}

func tapMessageTo(w io.Writer, m *tap.Message) error {
	frame, err := msg.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal: %s", err)
	}
	_, err = w.Write(frame)
	return err
}

func (h Dnstap) TapMessage(m *tap.Message) error {
	return tapMessageTo(h.Out, m)
}

func (h Dnstap) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	rw := &taprw.ResponseWriter{ResponseWriter: w, Taper: &h, Query: r, Pack: h.Pack}
	rw.QueryEpoch()

	code, err := middleware.NextOrFailure(h.Name(), h.Next, ctx, rw, r)
	if err != nil {
		// ignore dnstap errors
		return code, err
	}

	if err := rw.DnstapError(); err != nil {
		return code, middleware.Error("dnstap", err)
	}

	return code, nil
}
func (h Dnstap) Name() string { return "dnstap" }
