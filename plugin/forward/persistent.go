package forward

import (
	"crypto/tls"
	"sort"
	"time"

	"github.com/miekg/dns"
)

// a persistConn hold the dns.Conn and the last used time.
type persistConn struct {
	c    *dns.Conn
	used time.Time
}

// Transport hold the persistent cache.
type Transport struct {
	avgDialTime int64                          // kind of average time of dial time
	conns       [typeTotalCount][]*persistConn // Buckets for udp, tcp and tcp-tls.
	expire      time.Duration                  // After this duration a connection is expired.
	addr        string
	tlsConfig   *tls.Config

	dial  chan string
	yield chan *persistConn
	ret   chan *persistConn
	stop  chan bool
}

func newTransport(addr string) *Transport {
	t := &Transport{
		avgDialTime: int64(maxDialTimeout / 2),
		conns:       [typeTotalCount][]*persistConn{},
		expire:      defaultExpire,
		addr:        addr,
		dial:        make(chan string),
		yield:       make(chan *persistConn),
		ret:         make(chan *persistConn),
		stop:        make(chan bool),
	}
	return t
}

// connManagers manages the persistent connection cache for UDP and TCP.
func (t *Transport) connManager() {
	ticker := time.NewTicker(t.expire)
Wait:
	for {
		select {
		case proto := <-t.dial:
			transtype := stringToTransportType(proto)
			// take the last used conn - complexity O(1)
			if stack := t.conns[transtype]; len(stack) > 0 {
				pc := stack[len(stack)-1]
				if time.Since(pc.used) < t.expire {
					// Found one, remove from pool and return this conn.
					t.conns[transtype] = stack[:len(stack)-1]
					t.ret <- pc
					continue Wait
				}
				// clear entire cache if the last conn is expired
				t.conns[transtype] = nil
				// now, the connections being passed to closeConns() are not reachable from
				// transport methods anymore. So, it's safe to close them in a separate goroutine
				go closeConns(stack)
			}
			t.ret <- nil

		case pc := <-t.yield:
			transtype := t.transportTypeFromConn(pc)
			t.conns[transtype] = append(t.conns[transtype], pc)

		case <-ticker.C:
			t.cleanup(false)

		case <-t.stop:
			t.cleanup(true)
			close(t.ret)
			return
		}
	}
}

// closeConns closes connections.
func closeConns(conns []*persistConn) {
	for _, pc := range conns {
		pc.c.Close()
	}
}

// cleanup removes connections from cache.
func (t *Transport) cleanup(all bool) {
	staleTime := time.Now().Add(-t.expire)
	for transtype, stack := range t.conns {
		if len(stack) == 0 {
			continue
		}
		if all {
			t.conns[transtype] = nil
			// now, the connections being passed to closeConns() are not reachable from
			// transport methods anymore. So, it's safe to close them in a separate goroutine
			go closeConns(stack)
			continue
		}
		if stack[0].used.After(staleTime) {
			continue
		}

		// connections in stack are sorted by "used"
		good := sort.Search(len(stack), func(i int) bool {
			return stack[i].used.After(staleTime)
		})
		t.conns[transtype] = stack[good:]
		// now, the connections being passed to closeConns() are not reachable from
		// transport methods anymore. So, it's safe to close them in a separate goroutine
		go closeConns(stack[:good])
	}
}

// It is hard to pin a value to this, the import thing is to no block forever, losing at cached connection is not terrible.
const yieldTimeout = 25 * time.Millisecond

// Yield return the connection to transport for reuse.
func (t *Transport) Yield(pc *persistConn) {
	pc.used = time.Now() // update used time

	// Make this non-blocking, because in the case of a very busy forwarder we will *block* on this yield. This
	// blocks the outer go-routine and stuff will just pile up.  We timeout when the send fails to as returning
	// these connection is an optimization anyway.
	select {
	case t.yield <- pc:
		return
	case <-time.After(yieldTimeout):
		return
	}
}

// Start starts the transport's connection manager.
func (t *Transport) Start() { go t.connManager() }

// Stop stops the transport's connection manager.
func (t *Transport) Stop() { close(t.stop) }

// SetExpire sets the connection expire time in transport.
func (t *Transport) SetExpire(expire time.Duration) { t.expire = expire }

// SetTLSConfig sets the TLS config in transport.
func (t *Transport) SetTLSConfig(cfg *tls.Config) { t.tlsConfig = cfg }

const (
	defaultExpire  = 10 * time.Second
	minDialTimeout = 1 * time.Second
	maxDialTimeout = 30 * time.Second

	// Some resolves might take quite a while, usually (cached) responses are fast. Set to 2s to give us some time to retry a different upstream.
	readTimeout = 2 * time.Second
)
