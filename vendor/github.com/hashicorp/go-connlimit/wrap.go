package connlimit

import "net"

// WrappedConn wraps a net.Conn and free() func returned by Limiter.Accept so
// that when the wrapped connections Close method is called, its free func is
// also called.
type WrappedConn struct {
	net.Conn
	free func()
}

// Wrap wraps a net.Conn's Close method so free() is called when Close is
// called. Useful when handing off tracked connections to libraries that close
// them.
func Wrap(conn net.Conn, free func()) net.Conn {
	return &WrappedConn{
		Conn: conn,
		free: free,
	}
}

// Close frees the tracked connection and closes the underlying net.Conn.
func (w *WrappedConn) Close() error {
	w.free()
	return w.Conn.Close()
}
