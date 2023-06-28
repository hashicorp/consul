// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxy

import (
	"io"
	"net"
	"sync/atomic"
)

// Conn represents a single proxied TCP connection.
type Conn struct {
	src, dst net.Conn
	// TODO(banks): benchmark and consider adding _ [8]uint64 padding between
	// these to prevent false sharing between the rx and tx goroutines when
	// running on separate cores.
	srcW, dstW countWriter
	stopping   int32
}

// NewConn returns a conn joining the two given net.Conn
func NewConn(src, dst net.Conn) *Conn {
	return &Conn{
		src:      src,
		dst:      dst,
		srcW:     countWriter{w: src},
		dstW:     countWriter{w: dst},
		stopping: 0,
	}
}

// Close closes both connections.
func (c *Conn) Close() error {
	// Note that net.Conn.Close can be called multiple times and atomic store is
	// idempotent so no need to ensure we only do this once.
	//
	// Also note that we don't wait for CopyBytes to return here since we are
	// closing the conns which is the only externally visible sideeffect of that
	// goroutine running and there should be no way for it to hang or leak once
	// the conns are closed so we can save the extra coordination.
	atomic.StoreInt32(&c.stopping, 1)
	c.src.Close()
	c.dst.Close()
	return nil
}

// CopyBytes will continuously copy bytes in both directions between src and dst
// until either connection is closed.
func (c *Conn) CopyBytes() error {
	defer c.Close()

	go func() {
		// Need this since Copy is only guaranteed to stop when it's source reader
		// (second arg) hits EOF or error but either conn might close first possibly
		// causing this goroutine to exit but not the outer one. See
		// TestConnSrcClosing which will fail if you comment the defer below.
		defer c.Close()
		io.Copy(&c.dstW, c.src)
	}()

	_, err := io.Copy(&c.srcW, c.dst)
	// Note that we don't wait for the other goroutine to finish because it either
	// already has due to it's src conn closing, or it will once our defer fires
	// and closes the source conn. No need for the extra coordination.
	if atomic.LoadInt32(&c.stopping) == 1 {
		return nil
	}
	return err
}

// Stats returns number of bytes transmitted and received. Transmit means bytes
// written to dst, receive means bytes written to src.
func (c *Conn) Stats() (txBytes, rxBytes uint64) {
	return c.srcW.Written(), c.dstW.Written()
}

// countWriter is an io.Writer that counts the number of bytes being written
// before passing them through. We use it to gather metrics for bytes
// sent/received. Note that since we are always copying between a net.TCPConn
// and a tls.Conn, none of the optimisations using syscalls like splice and
// ReaderTo/WriterFrom can be used anyway and io.Copy falls back to a generic
// buffered read/write loop.
//
// We use atomic updates to synchronize reads and writes here. It's the cheapest
// uncontended option based on
// https://gist.github.com/banks/e76b40c0cc4b01503f0a0e4e0af231d5. Further
// optimization can be made when if/when identified as a real overhead.
type countWriter struct {
	written uint64
	w       io.Writer
}

// Write implements io.Writer
func (cw *countWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	atomic.AddUint64(&cw.written, uint64(n))
	return
}

// Written returns how many bytes have been written to w.
func (cw *countWriter) Written() uint64 {
	return atomic.LoadUint64(&cw.written)
}
