// Package taprw takes a query and intercepts the response.
// It will log both after the response is written.
package taprw

import (
	"fmt"
	"time"

	"github.com/coredns/coredns/plugin/dnstap/msg"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

// SendOption stores the flag to indicate whether a certain DNSTap message to
// be sent out or not.
type SendOption struct {
	Cq bool
	Cr bool
}

// Tapper is what ResponseWriter needs to log to dnstap.
type Tapper interface {
	TapMessage(m *tap.Message) error
	TapBuilder() msg.Builder
}

// ResponseWriter captures the client response and logs the query to dnstap.
// Single request use.
// SendOption configures Dnstap to selectively send Dnstap messages. Default is send all.
type ResponseWriter struct {
	queryEpoch uint64
	Query      *dns.Msg
	dns.ResponseWriter
	Tapper
	err  error
	Send *SendOption
}

// DnstapError check if a dnstap error occurred during Write and returns it.
func (w ResponseWriter) DnstapError() error {
	return w.err
}

// SetQueryEpoch sets the query epoch as reported by dnstap.
func (w *ResponseWriter) SetQueryEpoch() {
	w.queryEpoch = uint64(time.Now().Unix())
}

// WriteMsg writes back the response to the client and THEN works on logging the request
// and response to dnstap.
// Dnstap errors are to be checked by DnstapError.
func (w *ResponseWriter) WriteMsg(resp *dns.Msg) (writeErr error) {
	writeErr = w.ResponseWriter.WriteMsg(resp)
	writeEpoch := uint64(time.Now().Unix())

	b := w.TapBuilder()
	b.TimeSec = w.queryEpoch

	if w.Send == nil || w.Send.Cq {
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
	}

	if w.Send == nil || w.Send.Cr {
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
	}
	return
}
