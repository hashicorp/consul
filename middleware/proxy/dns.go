package proxy

import (
	"net"
	"time"

	"github.com/miekg/coredns/middleware/pkg/singleflight"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

type dnsEx struct {
	Timeout time.Duration
	group   *singleflight.Group
}

func newDNSEx() *dnsEx {
	return &dnsEx{group: new(singleflight.Group), Timeout: defaultTimeout * time.Second}
}

func (g *dnsEx) Protocol() string          { return "dns" }
func (d *dnsEx) OnShutdown(p *Proxy) error { return nil }
func (d *dnsEx) OnStartup(p *Proxy) error  { return nil }

// Exchange implements the Exchanger interface.
func (d *dnsEx) Exchange(addr string, state request.Request) (*dns.Msg, error) {
	co, err := net.DialTimeout(state.Proto(), addr, d.Timeout)
	if err != nil {
		return nil, err
	}

	reply, _, err := d.ExchangeConn(state.Req, co)

	co.Close()

	if reply != nil && reply.Truncated {
		// Suppress proxy error for truncated responses
		err = nil
	}

	if err != nil {
		return nil, err
	}

	reply.Compress = true
	reply.Id = state.Req.Id

	return reply, nil
}

func (d *dnsEx) ExchangeConn(m *dns.Msg, co net.Conn) (*dns.Msg, time.Duration, error) {
	t := "nop"
	if t1, ok := dns.TypeToString[m.Question[0].Qtype]; ok {
		t = t1
	}
	cl := "nop"
	if cl1, ok := dns.ClassToString[m.Question[0].Qclass]; ok {
		cl = cl1
	}

	start := time.Now()

	// Name needs to be normalized! Bug in go dns.
	r, err := d.group.Do(m.Question[0].Name+t+cl, func() (interface{}, error) {
		return exchange(m, co)
	})

	r1 := r.(dns.Msg)
	rtt := time.Since(start)
	return &r1, rtt, err
}

// exchange does *not* return a pointer to dns.Msg because that leads to buffer reuse when
// group.Do is used in Exchange.
func exchange(m *dns.Msg, co net.Conn) (dns.Msg, error) {
	opt := m.IsEdns0()

	udpsize := uint16(dns.MinMsgSize)
	// If EDNS0 is used use that for size.
	if opt != nil && opt.UDPSize() >= dns.MinMsgSize {
		udpsize = opt.UDPSize()
	}

	dnsco := &dns.Conn{Conn: co, UDPSize: udpsize}

	writeDeadline := time.Now().Add(defaultTimeout)
	dnsco.SetWriteDeadline(writeDeadline)
	dnsco.WriteMsg(m)

	readDeadline := time.Now().Add(defaultTimeout)
	co.SetReadDeadline(readDeadline)
	r, err := dnsco.ReadMsg()

	dnsco.Close()
	if r == nil {
		return dns.Msg{}, err
	}
	return *r, err
}
