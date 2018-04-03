package proxy

import (
	"io"
	"net"
	"sync/atomic"
)

// Conn represents a single proxied TCP connection.
type Conn struct {
	src, dst net.Conn
	stopping int32
}

// NewConn returns a conn joining the two given net.Conn
func NewConn(src, dst net.Conn) *Conn {
	return &Conn{
		src:      src,
		dst:      dst,
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
		io.Copy(c.dst, c.src)
	}()

	_, err := io.Copy(c.src, c.dst)
	// Note that we don't wait for the other goroutine to finish because it either
	// already has due to it's src conn closing, or it will once our defer fires
	// and closes the source conn. No need for the extra coordination.
	if atomic.LoadInt32(&c.stopping) == 1 {
		return nil
	}
	return err
}
