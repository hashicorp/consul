package forward

import (
	"net"
	"time"

	"github.com/miekg/dns"
)

// a persistConn hold the dns.Conn and the last used time.
type persistConn struct {
	c    *dns.Conn
	used time.Time
}

// connErr is used to communicate the connection manager.
type connErr struct {
	c   *dns.Conn
	err error
}

// transport hold the persistent cache.
type transport struct {
	conns map[string][]*persistConn //  Buckets for udp, tcp and tcp-tls.
	host  *host

	dial  chan string
	yield chan connErr
	ret   chan connErr

	// Aid in testing, gets length of cache in data-race safe manner.
	lenc    chan bool
	lencOut chan int

	stop chan bool
}

func newTransport(h *host) *transport {
	t := &transport{
		conns:   make(map[string][]*persistConn),
		host:    h,
		dial:    make(chan string),
		yield:   make(chan connErr),
		ret:     make(chan connErr),
		stop:    make(chan bool),
		lenc:    make(chan bool),
		lencOut: make(chan int),
	}
	go t.connManager()
	return t
}

// len returns the number of connection, used for metrics. Can only be safely
// used inside connManager() because of races.
func (t *transport) len() int {
	l := 0
	for _, conns := range t.conns {
		l += len(conns)
	}
	return l
}

// Len returns the number of connections in the cache.
func (t *transport) Len() int {
	t.lenc <- true
	l := <-t.lencOut
	return l
}

// connManagers manages the persistent connection cache for UDP and TCP.
func (t *transport) connManager() {

Wait:
	for {
		select {
		case proto := <-t.dial:
			// Yes O(n), shouldn't put millions in here. We walk all connection until we find the first
			// one that is usuable.
			i := 0
			for i = 0; i < len(t.conns[proto]); i++ {
				pc := t.conns[proto][i]
				if time.Since(pc.used) < t.host.expire {
					// Found one, remove from pool and return this conn.
					t.conns[proto] = t.conns[proto][i+1:]
					t.ret <- connErr{pc.c, nil}
					continue Wait
				}
				// This conn has expired. Close it.
				pc.c.Close()
			}

			// Not conns were found. Connect to the upstream to create one.
			t.conns[proto] = t.conns[proto][i:]
			SocketGauge.WithLabelValues(t.host.addr).Set(float64(t.len()))

			go func() {
				if proto != "tcp-tls" {
					c, err := dns.DialTimeout(proto, t.host.addr, dialTimeout)
					t.ret <- connErr{c, err}
					return
				}

				c, err := dns.DialTimeoutWithTLS("tcp", t.host.addr, t.host.tlsConfig, dialTimeout)
				t.ret <- connErr{c, err}
			}()

		case conn := <-t.yield:

			SocketGauge.WithLabelValues(t.host.addr).Set(float64(t.len() + 1))

			// no proto here, infer from config and conn
			if _, ok := conn.c.Conn.(*net.UDPConn); ok {
				t.conns["udp"] = append(t.conns["udp"], &persistConn{conn.c, time.Now()})
				continue Wait
			}

			if t.host.tlsConfig == nil {
				t.conns["tcp"] = append(t.conns["tcp"], &persistConn{conn.c, time.Now()})
				continue Wait
			}

			t.conns["tcp-tls"] = append(t.conns["tcp-tls"], &persistConn{conn.c, time.Now()})

		case <-t.stop:
			return

		case <-t.lenc:
			l := 0
			for _, conns := range t.conns {
				l += len(conns)
			}
			t.lencOut <- l
		}
	}
}

func (t *transport) Dial(proto string) (*dns.Conn, error) {
	t.dial <- proto
	c := <-t.ret
	return c.c, c.err
}

func (t *transport) Yield(c *dns.Conn) {
	t.yield <- connErr{c, nil}
}

// Stop stops the transports.
func (t *transport) Stop() { t.stop <- true }
