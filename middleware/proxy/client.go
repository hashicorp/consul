package proxy

import (
	"net"
	"time"

	"github.com/miekg/coredns/middleware/pkg/singleflight"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

type client struct {
	Timeout time.Duration

	group *singleflight.Group
}

func newClient() *client {
	return &client{Timeout: defaultTimeout, group: new(singleflight.Group)}
}

// ServeDNS does not satisfy middleware.Handler, instead it interacts with the upstream
// and returns the respons or an error.
func (c *client) ServeDNS(w dns.ResponseWriter, r *dns.Msg, u *UpstreamHost) (*dns.Msg, error) {
	co, err := net.DialTimeout(request.Proto(w), u.Name, c.Timeout)
	if err != nil {
		return nil, err
	}

	reply, _, err := c.Exchange(r, co)

	co.Close()

	if reply != nil && reply.Truncated {
		// Suppress proxy error for truncated responses
		err = nil
	}

	if err != nil {
		return nil, err
	}

	reply.Compress = true
	reply.Id = r.Id

	return reply, nil
}

func (c *client) Exchange(m *dns.Msg, co net.Conn) (*dns.Msg, time.Duration, error) {
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
	r, err := c.group.Do(m.Question[0].Name+t+cl, func() (interface{}, error) {
		ret, e := c.exchange(m, co)
		return ret, e
	})

	rtt := time.Since(start)
	if err != nil {
		return &dns.Msg{}, rtt, err
	}

	r1 := r.(dns.Msg)
	return &r1, rtt, nil
}

// exchange does *not* return a pointer to dns.Msg because that leads to buffer reuse when
// group.Do is used in Exchange.
func (c *client) exchange(m *dns.Msg, co net.Conn) (dns.Msg, error) {
	opt := m.IsEdns0()

	udpsize := uint16(dns.MinMsgSize)
	// If EDNS0 is used use that for size.
	if opt != nil && opt.UDPSize() >= dns.MinMsgSize {
		udpsize = opt.UDPSize()
	}

	dnsco := &dns.Conn{Conn: co, UDPSize: udpsize}

	dnsco.WriteMsg(m)
	r, err := dnsco.ReadMsg()
	dnsco.Close()
	if r == nil {
		return dns.Msg{}, err
	}
	return *r, err
}
