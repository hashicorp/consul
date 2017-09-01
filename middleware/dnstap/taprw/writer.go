// Package taprw takes a query and intercepts the response.
// It will log both after the response is written.
package taprw

import (
	"fmt"

	"github.com/coredns/coredns/middleware/dnstap/msg"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

// Tapper is what ResponseWriter needs to log to dnstap.
type Tapper interface {
	TapMessage(m *tap.Message) error
	TapBuilder() msg.Builder
}

// ResponseWriter captures the client response and logs the query to dnstap.
// Single request use.
type ResponseWriter struct {
	queryEpoch uint64
	Query      *dns.Msg
	dns.ResponseWriter
	Tapper
	err error
}

// DnstapError check if a dnstap error occurred during Write and returns it.
func (w ResponseWriter) DnstapError() error {
	return w.err
}

// QueryEpoch sets the query epoch as reported by dnstap.
func (w *ResponseWriter) QueryEpoch() {
	w.queryEpoch = msg.Epoch()
}

// WriteMsg writes back the response to the client and THEN works on logging the request
// and response to dnstap.
// Dnstap errors are to be checked by DnstapError.
func (w *ResponseWriter) WriteMsg(resp *dns.Msg) (writeErr error) {
	writeErr = w.ResponseWriter.WriteMsg(resp)
	writeEpoch := msg.Epoch()

	b := w.TapBuilder()
	b.TimeSec = w.queryEpoch
	if err := func() (err error) {
		err = b.AddrMsg(w.ResponseWriter.RemoteAddr(), w.Query)
		if err != nil {
			return
		}
		return w.TapMessage(b.ToClientQuery())
	}(); err != nil {
		w.err = fmt.Errorf("client query: %s", err)
		// don't forget to call DnstapError later
	}

	if writeErr == nil {
		if err := func() (err error) {
			b.TimeSec = writeEpoch
			if err = b.Msg(resp); err != nil {
				return
			}
			return w.TapMessage(b.ToClientResponse())
		}(); err != nil {
			w.err = fmt.Errorf("client response: %s", err)
		}
	}

	return
}
