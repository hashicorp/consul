package dnstest

import (
	"time"

	"github.com/miekg/dns"
)

// MultiRecorder is a type of ResponseWriter that captures all messages written to it.
type MultiRecorder struct {
	Len   int
	Msgs  []*dns.Msg
	Start time.Time
	dns.ResponseWriter
}

// NewMultiRecorder makes and returns a new MultiRecorder.
func NewMultiRecorder(w dns.ResponseWriter) *MultiRecorder {
	return &MultiRecorder{
		ResponseWriter: w,
		Msgs:           make([]*dns.Msg, 0),
		Start:          time.Now(),
	}
}

// WriteMsg records the message and its length written to it and call the
// underlying ResponseWriter's WriteMsg method.
func (r *MultiRecorder) WriteMsg(res *dns.Msg) error {
	r.Len += res.Len()
	r.Msgs = append(r.Msgs, res)
	return r.ResponseWriter.WriteMsg(res)
}

// Write is a wrapper that records the length of the messages that get written to it.
func (r *MultiRecorder) Write(buf []byte) (int, error) {
	n, err := r.ResponseWriter.Write(buf)
	if err == nil {
		r.Len += n
	}
	return n, err
}
