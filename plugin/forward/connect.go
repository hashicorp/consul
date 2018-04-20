// Package forward implements a forwarding proxy. It caches an upstream net.Conn for some time, so if the same
// client returns the upstream's Conn will be precached. Depending on how you benchmark this looks to be
// 50% faster than just openening a new connection for every client. It works with UDP and TCP and uses
// inband healthchecking.
package forward

import (
	"io"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/request"

	"context"

	"github.com/miekg/dns"
)

func (p *Proxy) readTimeout() time.Duration {
	rtt := time.Duration(atomic.LoadInt64(&p.avgRtt))

	if rtt < minTimeout {
		return minTimeout
	}
	if rtt < maxTimeout/2 {
		return 2 * rtt
	}
	return maxTimeout
}

func (p *Proxy) updateRtt(newRtt time.Duration) {
	rtt := time.Duration(atomic.LoadInt64(&p.avgRtt))
	atomic.AddInt64(&p.avgRtt, int64((newRtt-rtt)/rttCount))
}

func (p *Proxy) connect(ctx context.Context, state request.Request, forceTCP, metric bool) (*dns.Msg, error) {
	start := time.Now()

	proto := state.Proto()
	if forceTCP {
		proto = "tcp"
	}

	conn, cached, err := p.Dial(proto)
	if err != nil {
		return nil, err
	}

	// Set buffer size correctly for this client.
	conn.UDPSize = uint16(state.Size())
	if conn.UDPSize < 512 {
		conn.UDPSize = 512
	}

	conn.SetWriteDeadline(time.Now().Add(timeout))
	reqTime := time.Now()
	if err := conn.WriteMsg(state.Req); err != nil {
		conn.Close() // not giving it back
		if err == io.EOF && cached {
			return nil, errCachedClosed
		}
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(p.readTimeout()))
	ret, err := conn.ReadMsg()
	if err != nil {
		p.updateRtt(timeout)
		conn.Close() // not giving it back
		if err == io.EOF && cached {
			return nil, errCachedClosed
		}
		return ret, err
	}

	p.updateRtt(time.Since(reqTime))

	p.Yield(conn)

	if metric {
		rc, ok := dns.RcodeToString[ret.Rcode]
		if !ok {
			rc = strconv.Itoa(ret.Rcode)
		}

		RequestCount.WithLabelValues(p.addr).Add(1)
		RcodeCount.WithLabelValues(rc, p.addr).Add(1)
		RequestDuration.WithLabelValues(p.addr).Observe(time.Since(start).Seconds())
	}

	return ret, nil
}

const rttCount = 4
