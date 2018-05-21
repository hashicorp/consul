// Package nonwriter implements a dns.ResponseWriter that never writes, but captures the dns.Msg being written.
package nonwriter

import (
	"net"

	"github.com/miekg/dns"
)

// Writer is a type of ResponseWriter that captures the message, but never writes to the client.
type Writer struct {
	dns.ResponseWriter
	Msg *dns.Msg

	// Raddr is the remote's address. This can be optionally set.
	Raddr net.Addr
	// Laddr is our address. This can be optionally set.
	Laddr net.Addr
}

// New makes and returns a new NonWriter.
func New(w dns.ResponseWriter) *Writer { return &Writer{ResponseWriter: w} }

// WriteMsg records the message, but doesn't write it itself.
func (w *Writer) WriteMsg(res *dns.Msg) error {
	w.Msg = res
	return nil
}

// RemoteAddr returns the remote address.
func (w *Writer) RemoteAddr() net.Addr { return w.Raddr }

// LocalAddr returns the local address.
func (w *Writer) LocalAddr() net.Addr { return w.Laddr }
