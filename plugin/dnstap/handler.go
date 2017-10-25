package dnstap

import (
	"fmt"
	"io"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/dnstap/msg"
	"github.com/coredns/coredns/plugin/dnstap/taprw"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Dnstap is the dnstap handler.
type Dnstap struct {
	Next plugin.Handler
	IO   IORoutine
	Pack bool
}

type (
	// IORoutine is the dnstap I/O thread as defined by: <http://dnstap.info/Architecture>.
	IORoutine interface {
		Dnstap(tap.Dnstap)
	}
	// Tapper is implemented by the Context passed by the dnstap handler.
	Tapper interface {
		TapMessage(*tap.Message) error
		TapBuilder() msg.Builder
	}
	tapContext struct {
		context.Context
		Dnstap
	}
)

// ContextKey defines the type of key that is used to save data into the context
type ContextKey string

const (
	// DnstapSendOption specifies the Dnstap message to be send.  Default is sent all.
	DnstapSendOption ContextKey = "dnstap-send-option"
)

// TapperFromContext will return a Tapper if the dnstap plugin is enabled.
func TapperFromContext(ctx context.Context) (t Tapper) {
	t, _ = ctx.(Tapper)
	return
}

func tapMessageTo(w io.Writer, m *tap.Message) error {
	frame, err := msg.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal: %s", err)
	}
	_, err = w.Write(frame)
	return err
}

// TapMessage implements Tapper.
func (h Dnstap) TapMessage(m *tap.Message) error {
	h.IO.Dnstap(msg.Wrap(m))
	return nil
}

// TapBuilder implements Tapper.
func (h Dnstap) TapBuilder() msg.Builder {
	return msg.Builder{Full: h.Pack}
}

// ServeDNS logs the client query and response to dnstap and passes the dnstap Context.
func (h Dnstap) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	// Add send option into context so other plugin can decide on which DNSTap
	// message to be sent out
	sendOption := taprw.SendOption{Cq: true, Cr: true}
	newCtx := context.WithValue(ctx, DnstapSendOption, &sendOption)

	rw := &taprw.ResponseWriter{ResponseWriter: w, Tapper: &h, Query: r, Send: &sendOption}
	rw.SetQueryEpoch()

	code, err := plugin.NextOrFailure(h.Name(), h.Next, tapContext{newCtx, h}, rw, r)
	if err != nil {
		// ignore dnstap errors
		return code, err
	}

	if err := rw.DnstapError(); err != nil {
		return code, plugin.Error("dnstap", err)
	}

	return code, nil
}

// Name returns dnstap.
func (h Dnstap) Name() string { return "dnstap" }
