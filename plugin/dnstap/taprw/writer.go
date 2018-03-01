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
	TapMessage(*tap.Message)
	Pack() bool
}

// ResponseWriter captures the client response and logs the query to dnstap.
// Single request use.
// SendOption configures Dnstap to selectively send Dnstap messages. Default is send all.
type ResponseWriter struct {
	QueryEpoch time.Time
	Query      *dns.Msg
	dns.ResponseWriter
	Tapper
	Send *SendOption

	dnstapErr error
}

// DnstapError check if a dnstap error occurred during Write and returns it.
func (w *ResponseWriter) DnstapError() error {
	return w.dnstapErr
}

// WriteMsg writes back the response to the client and THEN works on logging the request
// and response to dnstap.
func (w *ResponseWriter) WriteMsg(resp *dns.Msg) (writeErr error) {
	writeErr = w.ResponseWriter.WriteMsg(resp)
	writeEpoch := time.Now()

	b := msg.New().Time(w.QueryEpoch).Addr(w.RemoteAddr())

	if w.Send == nil || w.Send.Cq {
		if w.Pack() {
			b.Msg(w.Query)
		}
		if m, err := b.ToClientQuery(); err != nil {
			w.dnstapErr = fmt.Errorf("client query: %s", err)
		} else {
			w.TapMessage(m)
		}
	}

	if w.Send == nil || w.Send.Cr {
		if writeErr == nil {
			if w.Pack() {
				b.Msg(resp)
			}
			if m, err := b.Time(writeEpoch).ToClientResponse(); err != nil {
				w.dnstapErr = fmt.Errorf("client response: %s", err)
			} else {
				w.TapMessage(m)
			}
		}
	}

	return writeErr
}
