// Package request abstracts a client's request so that all plugins will handle them in an unified way.
package request

import (
	"context"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin/pkg/edns"

	"github.com/miekg/dns"
)

// Request contains some connection state and is useful in plugin.
type Request struct {
	Req *dns.Msg
	W   dns.ResponseWriter

	// Optional lowercased zone of this query.
	Zone string

	Context context.Context

	// Cache size after first call to Size or Do.
	size int
	do   *bool // nil: nothing, otherwise *do value
	// TODO(miek): opt record itself as well?

	// Caches
	name      string // lowercase qname.
	ip        string // client's ip.
	port      string // client's port.
	family    int    // transport's family.
	localPort string // server's port.
	localIP   string // server's ip.
}

// NewWithQuestion returns a new request based on the old, but with a new question
// section in the request.
func (r *Request) NewWithQuestion(name string, typ uint16) Request {
	req1 := Request{W: r.W, Req: r.Req.Copy()}
	req1.Req.Question[0] = dns.Question{Name: dns.Fqdn(name), Qclass: dns.ClassINET, Qtype: typ}
	return req1
}

// IP gets the (remote) IP address of the client making the request.
func (r *Request) IP() string {
	if r.ip != "" {
		return r.ip
	}

	ip, _, err := net.SplitHostPort(r.W.RemoteAddr().String())
	if err != nil {
		r.ip = r.W.RemoteAddr().String()
		return r.ip
	}

	r.ip = ip
	return r.ip
}

// LocalIP gets the (local) IP address of server handling the request.
func (r *Request) LocalIP() string {
	if r.localIP != "" {
		return r.localIP
	}

	ip, _, err := net.SplitHostPort(r.W.LocalAddr().String())
	if err != nil {
		r.localIP = r.W.LocalAddr().String()
		return r.localIP
	}

	r.localIP = ip
	return r.localIP
}

// Port gets the (remote) port of the client making the request.
func (r *Request) Port() string {
	if r.port != "" {
		return r.port
	}

	_, port, err := net.SplitHostPort(r.W.RemoteAddr().String())
	if err != nil {
		r.port = "0"
		return r.port
	}

	r.port = port
	return r.port
}

// LocalPort gets the local port of the server handling the request.
func (r *Request) LocalPort() string {
	if r.localPort != "" {
		return r.localPort
	}

	_, port, err := net.SplitHostPort(r.W.LocalAddr().String())
	if err != nil {
		r.localPort = "0"
		return r.localPort
	}

	r.localPort = port
	return r.localPort
}

// RemoteAddr returns the net.Addr of the client that sent the current request.
func (r *Request) RemoteAddr() string { return r.W.RemoteAddr().String() }

// LocalAddr returns the net.Addr of the server handling the current request.
func (r *Request) LocalAddr() string { return r.W.LocalAddr().String() }

// Proto gets the protocol used as the transport. This will be udp or tcp.
func (r *Request) Proto() string { return Proto(r.W) }

// Proto gets the protocol used as the transport. This will be udp or tcp.
func Proto(w dns.ResponseWriter) string {
	// FIXME(miek): why not a method on Request
	if _, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		return "udp"
	}
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		return "tcp"
	}
	return "udp"
}

// Family returns the family of the transport, 1 for IPv4 and 2 for IPv6.
func (r *Request) Family() int {
	if r.family != 0 {
		return r.family
	}

	var a net.IP
	ip := r.W.RemoteAddr()
	if i, ok := ip.(*net.UDPAddr); ok {
		a = i.IP
	}
	if i, ok := ip.(*net.TCPAddr); ok {
		a = i.IP
	}

	if a.To4() != nil {
		r.family = 1
		return r.family
	}
	r.family = 2
	return r.family
}

// Do returns if the request has the DO (DNSSEC OK) bit set.
func (r *Request) Do() bool {
	if r.do != nil {
		return *r.do
	}

	r.do = new(bool)

	if o := r.Req.IsEdns0(); o != nil {
		*r.do = o.Do()
		return *r.do
	}
	*r.do = false
	return false
}

// Len returns the length in bytes in the request.
func (r *Request) Len() int { return r.Req.Len() }

// Size returns if buffer size *advertised* in the requests OPT record.
// Or when the request was over TCP, we return the maximum allowed size of 64K.
func (r *Request) Size() int {
	if r.size != 0 {
		return r.size
	}

	size := 0
	if o := r.Req.IsEdns0(); o != nil {
		if r.do == nil {
			r.do = new(bool)
		}
		*r.do = o.Do()
		size = int(o.UDPSize())
	}

	size = edns.Size(r.Proto(), size)
	r.size = size
	return size
}

// SizeAndDo adds an OPT record that the reflects the intent from request.
// The returned bool indicates if an record was found and normalised.
func (r *Request) SizeAndDo(m *dns.Msg) bool {
	o := r.Req.IsEdns0()
	if o == nil {
		return false
	}

	if mo := m.IsEdns0(); mo != nil {
		mo.Hdr.Name = "."
		mo.Hdr.Rrtype = dns.TypeOPT
		mo.SetVersion(0)
		mo.SetUDPSize(o.UDPSize())
		mo.Hdr.Ttl &= 0xff00 // clear flags

		// Assume if the message m has options set, they are OK and represent what an upstream can do.

		if o.Do() {
			mo.SetDo()
		}
		return true
	}

	// Reuse the request's OPT record and tack it to m.
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	o.SetVersion(0)
	o.Hdr.Ttl &= 0xff00 // clear flags

	if len(o.Option) > 0 {
		o.Option = supportedOptions(o.Option)
	}

	m.Extra = append(m.Extra, o)
	return true
}

// Scrub scrubs the reply message so that it will fit the client's buffer. It will first
// check if the reply fits without compression and then *with* compression.
// Scrub will then use binary search to find a save cut off point in the additional section.
// If even *without* the additional section the reply still doesn't fit we
// repeat this process for the answer section. If we scrub the answer section
// we set the TC bit on the reply; indicating the client should retry over TCP.
// Note, the TC bit will be set regardless of protocol, even TCP message will
// get the bit, the client should then retry with pigeons.
func (r *Request) Scrub(reply *dns.Msg) *dns.Msg {
	size := r.Size()

	reply.Compress = false
	rl := reply.Len()
	if size >= rl {
		if r.Proto() != "udp" {
			return reply
		}

		// Last ditch attempt to avoid fragmentation, if the size is bigger than the v4/v6 UDP fragmentation
		// limit and sent via UDP compress it (in the hope we go under that limit). Limits taken from NSD:
		//
		//    .., 1480 (EDNS/IPv4), 1220 (EDNS/IPv6), or the advertized EDNS buffer size if that is
		//    smaller than the EDNS default.
		// See: https://open.nlnetlabs.nl/pipermail/nsd-users/2011-November/001278.html
		if rl > 1480 && r.Family() == 1 {
			reply.Compress = true
		}
		if rl > 1220 && r.Family() == 2 {
			reply.Compress = true
		}

		return reply
	}

	reply.Compress = true
	rl = reply.Len()
	if size >= rl {
		return reply
	}

	// Account for the OPT record that gets added in SizeAndDo(), subtract that length.
	re := len(reply.Extra)
	if r.Req.IsEdns0() != nil {
		size -= optLen
		// re can never be 0 because we have an OPT RR.
		re--
	}

	l, m := 0, 0
	origExtra := reply.Extra
	for l <= re {
		m = (l + re) / 2
		reply.Extra = origExtra[:m]
		rl = reply.Len()
		if rl < size {
			l = m + 1
			continue
		}
		if rl > size {
			re = m - 1
			continue
		}
		if rl == size {
			break
		}
	}

	// The binary search only breaks on an exact match, which will be
	// pretty rare. Normally, the loop will exit when l > re, meaning that
	// in the previous iteration either:
	// rl < size: no need to do anything.
	// rl > size: the final size is too large, and if m > 0, the preceding
	// iteration the size was too small. Select that preceding size.
	if rl > size && m > 0 {
		reply.Extra = origExtra[:m-1]
		rl = reply.Len()
	}

	if rl <= size {
		return reply
	}

	ra := len(reply.Answer)
	l, m = 0, 0
	origAnswer := reply.Answer
	for l <= ra {
		m = (l + ra) / 2
		reply.Answer = origAnswer[:m]
		rl = reply.Len()
		if rl < size {
			l = m + 1
			continue
		}
		if rl > size {
			ra = m - 1
			continue
		}
		if rl == size {
			break
		}
	}

	// The binary search only breaks on an exact match, which will be
	// pretty rare. Normally, the loop will exit when l > ra, meaning that
	// in the previous iteration either:
	// rl < size: no need to do anything.
	// rl > size: the final size is too large, and if m > 0, the preceding
	// iteration the size was too small. Select that preceding size.
	if rl > size && m > 0 {
		reply.Answer = origAnswer[:m-1]
		// No need to recalc length, as we don't use it. We set truncated anyway. Doing
		// this extra m-1 step does make it fit in the client's buffer however.
	}

	reply.Truncated = true
	return reply
}

// Type returns the type of the question as a string. If the request is malformed the empty string is returned.
func (r *Request) Type() string {
	if r.Req == nil {
		return ""
	}
	if len(r.Req.Question) == 0 {
		return ""
	}

	return dns.Type(r.Req.Question[0].Qtype).String()
}

// QType returns the type of the question as an uint16. If the request is malformed
// 0 is returned.
func (r *Request) QType() uint16 {
	if r.Req == nil {
		return 0
	}
	if len(r.Req.Question) == 0 {
		return 0
	}

	return r.Req.Question[0].Qtype
}

// Name returns the name of the question in the request. Note
// this name will always have a closing dot and will be lower cased. After a call Name
// the value will be cached. To clear this caching call Clear.
// If the request is malformed the root zone is returned.
func (r *Request) Name() string {
	if r.name != "" {
		return r.name
	}
	if r.Req == nil {
		r.name = "."
		return "."
	}
	if len(r.Req.Question) == 0 {
		r.name = "."
		return "."
	}

	r.name = strings.ToLower(dns.Name(r.Req.Question[0].Name).String())
	return r.name
}

// QName returns the name of the question in the request.
// If the request is malformed the root zone is returned.
func (r *Request) QName() string {
	if r.Req == nil {
		return "."
	}
	if len(r.Req.Question) == 0 {
		return "."
	}

	return dns.Name(r.Req.Question[0].Name).String()
}

// Class returns the class of the question in the request.
// If the request is malformed the empty string is returned.
func (r *Request) Class() string {
	if r.Req == nil {
		return ""
	}
	if len(r.Req.Question) == 0 {
		return ""
	}

	return dns.Class(r.Req.Question[0].Qclass).String()

}

// QClass returns the class of the question in the request.
// If the request is malformed 0 returned.
func (r *Request) QClass() uint16 {
	if r.Req == nil {
		return 0
	}
	if len(r.Req.Question) == 0 {
		return 0
	}

	return r.Req.Question[0].Qclass

}

// Clear clears all caching from Request s.
func (r *Request) Clear() {
	r.name = ""
	r.ip = ""
	r.localIP = ""
	r.port = ""
	r.localPort = ""
	r.family = 0
}

// Match checks if the reply matches the qname and qtype from the request, it returns
// false when they don't match.
func (r *Request) Match(reply *dns.Msg) bool {
	if len(reply.Question) != 1 {
		return false
	}

	if reply.Response == false {
		return false
	}

	if strings.ToLower(reply.Question[0].Name) != r.Name() {
		return false
	}

	if reply.Question[0].Qtype != r.QType() {
		return false
	}

	return true
}

const optLen = 12 // OPT record length.
