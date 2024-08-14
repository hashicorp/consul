// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"net"
)

// BufferResponseWriter writes a DNS response to a byte buffer.
type BufferResponseWriter struct {
	// responseBuffer is the buffer that the response is written to.
	responseBuffer []byte
	// RequestContext is the context of the request that carries the ACL token and tenancy of the request.
	RequestContext Context
	// LocalAddress is the address of the server.
	LocalAddress net.Addr
	// RemoteAddress is the address of the client that sent the request.
	RemoteAddress net.Addr
	// Logger is the logger for the response writer.
	Logger hclog.Logger
}

var _ dns.ResponseWriter = (*BufferResponseWriter)(nil)

// ResponseBuffer returns the buffer containing the response.
func (b *BufferResponseWriter) ResponseBuffer() []byte {
	return b.responseBuffer
}

// LocalAddr returns the net.Addr of the server
func (b *BufferResponseWriter) LocalAddr() net.Addr {
	return b.LocalAddress
}

// RemoteAddr returns the net.Addr of the client that sent the current request.
func (b *BufferResponseWriter) RemoteAddr() net.Addr {
	return b.RemoteAddress
}

// WriteMsg writes a reply back to the client.
func (b *BufferResponseWriter) WriteMsg(m *dns.Msg) error {
	// Pack message to bytes first.
	msgBytes, err := m.Pack()
	if err != nil {
		b.Logger.Error("error packing message", "err", err)
		return err
	}
	b.responseBuffer = msgBytes
	return nil
}

// Write writes a raw buffer back to the client.
func (b *BufferResponseWriter) Write(m []byte) (int, error) {
	b.Logger.Trace("Write was called")
	return copy(b.responseBuffer, m), nil
}

// Close closes the connection.
func (b *BufferResponseWriter) Close() error {
	// There's nothing for us to do here as we don't handle the connection.
	return nil
}

// TsigStatus returns the status of the Tsig.
func (b *BufferResponseWriter) TsigStatus() error {
	// TSIG doesn't apply to this response writer.
	return nil
}

// TsigTimersOnly sets the tsig timers only boolean.
func (b *BufferResponseWriter) TsigTimersOnly(bool) {}

// Hijack lets the caller take over the connection.
// After a call to Hijack(), the DNS package will not do anything with the connection. {
func (b *BufferResponseWriter) Hijack() {}
