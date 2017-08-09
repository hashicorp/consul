package autopath

import (
	"github.com/miekg/dns"
)

// NonWriter is a type of ResponseWriter that captures the message, but never writes to the client.
type NonWriter struct {
	dns.ResponseWriter
	Msg *dns.Msg
}

// NewNonWriter makes and returns a new NonWriter.
func NewNonWriter(w dns.ResponseWriter) *NonWriter { return &NonWriter{ResponseWriter: w} }

// WriteMsg records the message, but doesn't write it itself.
func (r *NonWriter) WriteMsg(res *dns.Msg) error {
	r.Msg = res
	return nil
}

func (r *NonWriter) Write(buf []byte) (int, error) { return len(buf), nil }
