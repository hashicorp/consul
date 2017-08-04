// Package taprw takes a query and intercepts the response.
// It will log both after the response is written.
package taprw

import (
	"fmt"

	"github.com/coredns/coredns/middleware/dnstap/msg"
	"github.com/coredns/coredns/request"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

type Taper interface {
	TapMessage(m *tap.Message) error
}

// Single request use.
type ResponseWriter struct {
	queryData msg.Data
	Query     *dns.Msg
	dns.ResponseWriter
	Taper
	Pack bool
	err  error
}

// Check if a dnstap error occurred.
// Set during ResponseWriter.Write.
func (w ResponseWriter) DnstapError() error {
	return w.err
}

// To be called as soon as possible.
func (w *ResponseWriter) QueryEpoch() {
	w.queryData.Epoch()
}

// Write back the response to the client and THEN work on logging the request
// and response to dnstap.
// Dnstap errors to be checked by DnstapError.
func (w *ResponseWriter) WriteMsg(resp *dns.Msg) error {
	writeErr := w.ResponseWriter.WriteMsg(resp)

	if err := tapQuery(w); err != nil {
		w.err = fmt.Errorf("client query: %s", err)
		// don't forget to call DnstapError later
	}

	if writeErr == nil {
		if err := tapResponse(w, resp); err != nil {
			w.err = fmt.Errorf("client response: %s", err)
		}
	}

	return writeErr
}
func tapQuery(w *ResponseWriter) error {
	req := request.Request{W: w.ResponseWriter, Req: w.Query}
	if err := w.queryData.FromRequest(req); err != nil {
		return err
	}
	if w.Pack {
		if err := w.queryData.Pack(w.Query); err != nil {
			return fmt.Errorf("pack: %s", err)
		}
	}
	return w.Taper.TapMessage(w.queryData.ToClientQuery())
}
func tapResponse(w *ResponseWriter, resp *dns.Msg) error {
	d := &msg.Data{}
	d.Epoch()
	req := request.Request{W: w, Req: resp}
	if err := d.FromRequest(req); err != nil {
		return err
	}
	if w.Pack {
		if err := d.Pack(resp); err != nil {
			return fmt.Errorf("pack: %s", err)
		}
	}
	return w.Taper.TapMessage(d.ToClientResponse())
}
