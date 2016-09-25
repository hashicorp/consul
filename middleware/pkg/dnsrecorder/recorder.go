// Package dnsrecorder allows you to record a DNS response when it is send to the client.
package dnsrecorder

import (
	"time"

	"github.com/miekg/dns"
)

// Recorder is a type of ResponseWriter that captures
// the rcode code written to it and also the size of the message
// written in the response. A rcode code does not have
// to be written, however, in which case 0 must be assumed.
// It is best to have the constructor initialize this type
// with that default status code.
type Recorder struct {
	dns.ResponseWriter
	Rcode int
	Size  int
	Msg   *dns.Msg
	Start time.Time
}

// New makes and returns a new Recorder,
// which captures the DNS rcode from the ResponseWriter
// and also the length of the response message written through it.
func New(w dns.ResponseWriter) *Recorder {
	return &Recorder{
		ResponseWriter: w,
		Rcode:          0,
		Msg:            nil,
		Start:          time.Now(),
	}
}

// WriteMsg records the status code and calls the
// underlying ResponseWriter's WriteMsg method.
func (r *Recorder) WriteMsg(res *dns.Msg) error {
	r.Rcode = res.Rcode
	// We may get called multiple times (axfr for instance).
	// Save the last message, but add the sizes.
	r.Size += res.Len()
	r.Msg = res
	return r.ResponseWriter.WriteMsg(res)
}

// Write is a wrapper that records the size of the message that gets written.
func (r *Recorder) Write(buf []byte) (int, error) {
	n, err := r.ResponseWriter.Write(buf)
	if err == nil {
		r.Size += n
	}
	return n, err
}

// Hijack implements dns.Hijacker. It simply wraps the underlying
// ResponseWriter's Hijack method if there is one, or returns an error.
func (r *Recorder) Hijack() { r.ResponseWriter.Hijack(); return }
