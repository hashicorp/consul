// Package nonwriter implements a dns.ResponseWriter that never writes, but captures the dns.Msg being written.
package nonwriter

import (
	"github.com/miekg/dns"
)

// Writer is a type of ResponseWriter that captures the message, but never writes to the client.
type Writer struct {
	dns.ResponseWriter
	Msg *dns.Msg
}

// New makes and returns a new NonWriter.
func New(w dns.ResponseWriter) *Writer { return &Writer{ResponseWriter: w} }

// WriteMsg records the message, but doesn't write it itself.
func (w *Writer) WriteMsg(res *dns.Msg) error {
	w.Msg = res
	return nil
}

func (w *Writer) Write(buf []byte) (int, error) { return len(buf), nil }
