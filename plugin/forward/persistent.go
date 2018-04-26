package forward

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/miekg/dns"
)

// a persistConn hold the dns.Conn and the last used time.
type persistConn struct {
	c    *dns.Conn
	used time.Time
}

// transport hold the persistent cache.
type transport struct {
	conns     map[string][]*persistConn //  Buckets for udp, tcp and tcp-tls.
	expire    time.Duration             // After this duration a connection is expired.
	addr      string
	tlsConfig *tls.Config

	dial  chan string
	yield chan *dns.Conn
	ret   chan *dns.Conn
	stop  chan bool
}

func newTransport(addr string, tlsConfig *tls.Config) *transport {
	t := &transport{
		conns:  make(map[string][]*persistConn),
		expire: defaultExpire,
		addr:   addr,
		dial:   make(chan string),
		yield:  make(chan *dns.Conn),
		ret:    make(chan *dns.Conn),
		stop:   make(chan bool),
	}
	go func() { t.connManager() }()
	return t
}

// len returns the number of connection, used for metrics. Can only be safely
// used inside connManager() because of data races.
func (t *transport) len() int {
	l := 0
	for _, conns := range t.conns {
		l += len(conns)
	}
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
				if time.Since(pc.used) < t.expire {
					// Found one, remove from pool and return this conn.
					t.conns[proto] = t.conns[proto][i+1:]
					t.ret <- pc.c
					continue Wait
				}
				// This conn has expired. Close it.
				pc.c.Close()
			}

			// Not conns were found. Connect to the upstream to create one.
			t.conns[proto] = t.conns[proto][i:]
			SocketGauge.WithLabelValues(t.addr).Set(float64(t.len()))

			t.ret <- nil

		case conn := <-t.yield:

			SocketGauge.WithLabelValues(t.addr).Set(float64(t.len() + 1))

			// no proto here, infer from config and conn
			if _, ok := conn.Conn.(*net.UDPConn); ok {
				t.conns["udp"] = append(t.conns["udp"], &persistConn{conn, time.Now()})
				continue Wait
			}

			if t.tlsConfig == nil {
				t.conns["tcp"] = append(t.conns["tcp"], &persistConn{conn, time.Now()})
				continue Wait
			}

			t.conns["tcp-tls"] = append(t.conns["tcp-tls"], &persistConn{conn, time.Now()})

		case <-t.stop:
			close(t.ret)
			return
		}
	}
}

// Dial dials the address configured in transport, potentially reusing a connection or creating a new one.
func (t *transport) Dial(proto string) (*dns.Conn, bool, error) {
	// If tls has been configured; use it.
	if t.tlsConfig != nil {
		proto = "tcp-tls"
	}

	t.dial <- proto
	c := <-t.ret

	if c != nil {
		return c, true, nil
	}

	if proto == "tcp-tls" {
		conn, err := dns.DialTimeoutWithTLS("tcp", t.addr, t.tlsConfig, dialTimeout)
		return conn, false, err
	}
	conn, err := dns.DialTimeout(proto, t.addr, dialTimeout)
	return conn, false, err
}

// Yield return the connection to transport for reuse.
func (t *transport) Yield(c *dns.Conn) { t.yield <- c }

// Stop stops the transport's connection manager.
func (t *transport) Stop() { close(t.stop) }

// SetExpire sets the connection expire time in transport.
func (t *transport) SetExpire(expire time.Duration) { t.expire = expire }

// SetTLSConfig sets the TLS config in transport.
func (t *transport) SetTLSConfig(cfg *tls.Config) { t.tlsConfig = cfg }

const defaultExpire = 10 * time.Second
