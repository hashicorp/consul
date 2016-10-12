// Package request abstracts a client's request so that all middleware
// will handle them in an unified way.
package request

import (
	"net"
	"strings"

	"github.com/miekg/coredns/middleware/pkg/edns"

	"github.com/miekg/dns"
)

// Request contains some connection state and is useful in middleware.
type Request struct {
	Req *dns.Msg
	W   dns.ResponseWriter

	// Cache size after first call to Size or Do.
	size int
	do   int // 0: not, 1: true: 2: false
	// TODO(miek): opt record itself as well?

	// Cache lowercase qname.
	name string
}

// NewWithQuestion returns a new request based on the old, but with a new question
// section in the request.
func (r *Request) NewWithQuestion(name string, typ uint16) Request {
	req1 := Request{W: r.W, Req: r.Req.Copy()}
	req1.Req.Question[0] = dns.Question{Name: dns.Fqdn(name), Qtype: dns.ClassINET, Qclass: typ}
	return req1
}

// IP gets the (remote) IP address of the client making the request.
func (r *Request) IP() string {
	ip, _, err := net.SplitHostPort(r.W.RemoteAddr().String())
	if err != nil {
		return r.W.RemoteAddr().String()
	}
	return ip
}

// Port gets the (remote) Port of the client making the request.
func (r *Request) Port() string {
	_, port, err := net.SplitHostPort(r.W.RemoteAddr().String())
	if err != nil {
		return "0"
	}
	return port
}

// RemoteAddr returns the net.Addr of the client that sent the current request.
func (r *Request) RemoteAddr() string {
	return r.W.RemoteAddr().String()
}

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
	var a net.IP
	ip := r.W.RemoteAddr()
	if i, ok := ip.(*net.UDPAddr); ok {
		a = i.IP
	}
	if i, ok := ip.(*net.TCPAddr); ok {
		a = i.IP
	}

	if a.To4() != nil {
		return 1
	}
	return 2
}

// Do returns if the request has the DO (DNSSEC OK) bit set.
func (r *Request) Do() bool {
	if r.do != 0 {
		return r.do == doTrue
	}

	if o := r.Req.IsEdns0(); o != nil {
		if o.Do() {
			r.do = doTrue
		} else {
			r.do = doFalse
		}
		return o.Do()
	}
	r.do = doFalse
	return false
}

// Size returns if UDP buffer size advertised in the requests OPT record.
// Or when the request was over TCP, we return the maximum allowed size of 64K.
func (r *Request) Size() int {
	if r.size != 0 {
		return r.size
	}

	size := 0
	if o := r.Req.IsEdns0(); o != nil {
		if o.Do() == true {
			r.do = doTrue
		} else {
			r.do = doFalse
		}
		size = int(o.UDPSize())
	}
	// TODO(miek) move edns.Size to dnsutil?
	size = edns.Size(r.Proto(), size)
	r.size = size
	return size
}

// SizeAndDo adds an OPT record that the reflects the intent from request.
// The returned bool indicated if an record was found and normalised.
func (r *Request) SizeAndDo(m *dns.Msg) bool {
	o := r.Req.IsEdns0() // TODO(miek): speed this up
	if o == nil {
		return false
	}

	odo := o.Do()

	if mo := m.IsEdns0(); mo != nil {
		mo.Hdr.Name = "."
		mo.Hdr.Rrtype = dns.TypeOPT
		mo.SetVersion(0)
		mo.SetUDPSize(o.UDPSize())
		mo.Hdr.Ttl &= 0xff00 // clear flags

		if odo {
			mo.SetDo()
		}
		return true
	}

	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	o.SetVersion(0)
	o.Hdr.Ttl &= 0xff00 // clear flags

	if odo {
		o.SetDo()
	}

	m.Extra = append(m.Extra, o)
	return true
}

// Result is the result of Scrub.
type Result int

const (
	// ScrubIgnored is returned when Scrub did nothing to the message.
	ScrubIgnored Result = iota
	// ScrubDone is returned when the reply has been scrubbed.
	ScrubDone
)

// Scrub scrubs the reply message so that it will fit the client's buffer. If even after dropping
// the additional section, it still does not fit the TC bit will be set on the message. Note,
// the TC bit will be set regardless of protocol, even TCP message will get the bit, the client
// should then retry with pigeons.
// TODO(referral).
func (r *Request) Scrub(reply *dns.Msg) (*dns.Msg, Result) {
	size := r.Size()
	l := reply.Len()
	if size >= l {
		return reply, ScrubIgnored
	}
	// TODO(miek): check for delegation

	// If not delegation, drop additional section.
	reply.Extra = nil
	r.SizeAndDo(reply)
	l = reply.Len()
	if size >= l {
		return reply, ScrubDone
	}
	// Still?!! does not fit.
	reply.Truncated = true
	return reply, ScrubDone
}

// Type returns the type of the question as a string.
func (r *Request) Type() string { return dns.Type(r.Req.Question[0].Qtype).String() }

// QType returns the type of the question as an uint16.
func (r *Request) QType() uint16 { return r.Req.Question[0].Qtype }

// Name returns the name of the question in the request. Note
// this name will always have a closing dot and will be lower cased. After a call Name
// the value will be cached. To clear this caching call Clear.
func (r *Request) Name() string {
	if r.name != "" {
		return r.name
	}
	r.name = strings.ToLower(dns.Name(r.Req.Question[0].Name).String())
	return r.name
}

// QName returns the name of the question in the request.
func (r *Request) QName() string { return dns.Name(r.Req.Question[0].Name).String() }

// Class returns the class of the question in the request.
func (r *Request) Class() string { return dns.Class(r.Req.Question[0].Qclass).String() }

// QClass returns the class of the question in the request.
func (r *Request) QClass() uint16 { return r.Req.Question[0].Qclass }

// ErrorMessage returns an error message suitable for sending
// back to the client.
func (r *Request) ErrorMessage(rcode int) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(r.Req, rcode)
	return m
}

// Clear clears all caching from Request s.
func (r *Request) Clear() {
	r.name = ""
}

const (
	// TODO(miek): make this less awkward.
	doTrue  = 1
	doFalse = 2
)
