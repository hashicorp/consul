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
func (c *Conn) Close() {
	atomic.StoreInt32(&c.stopping, 1)
	c.src.Close()
	c.dst.Close()
}

// CopyBytes will continuously copy bytes in both directions between src and dst
// until either connection is closed.
func (c *Conn) CopyBytes() error {
	defer c.Close()

	go func() {
		// Need this since Copy is only guaranteed to stop when it's source reader
		// (second arg) hits EOF or error but either conn might close first possibly
		// causing this goroutine to exit but not the outer one. See TestSc
		//defer c.Close()
		io.Copy(c.dst, c.src)
	}()
	_, err := io.Copy(c.src, c.dst)
	if atomic.LoadInt32(&c.stopping) == 1 {
		return nil
	}
	return err
}
