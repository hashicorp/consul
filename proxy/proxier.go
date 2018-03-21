package proxy

import (
	"errors"
	"net"
)

// ErrStopped is returned for operations on a proxy that is stopped
var ErrStopped = errors.New("stopped")

// ErrStopping is returned for operations on a proxy that is stopping
var ErrStopping = errors.New("stopping")

// Proxier is an interface for managing different proxy implementations in a
// standard way. We have at least two different types of Proxier implementations
// needed: one for the incoming mTLS -> local proxy and another for each
// "upstream" service the app needs to talk out to (which listens locally and
// performs service discovery to find a suitable remote service).
type Proxier interface {
	// Listener returns a net.Listener that is open and ready for use, the Proxy
	// manager will arrange accepting new connections from it and passing them to
	// the handler method.
	Listener() (net.Listener, error)

	// HandleConn is called for each incoming connection accepted by the listener.
	// It is called in it's own goroutine and should run until it hits an error.
	// When stopping the Proxier, the manager will simply close the conn provided
	// and expects an error to be eventually returned. Any time spent not blocked
	// on the passed conn (for example doing service discovery) should therefore
	// be time-bound so that shutdown can't stall forever.
	HandleConn(conn net.Conn) error
}
