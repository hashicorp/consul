package middleware

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// This file contains the state nd functions available for use in the templates.

// State contains some connection state and is useful in middleware.
type State struct {
	Root http.FileSystem // TODO(miek): needed?
	Req  *dns.Msg
	W    dns.ResponseWriter
}

// Now returns the current timestamp in the specified format.
func (s State) Now(format string) string {
	return time.Now().Format(format)
}

// NowDate returns the current date/time that can be used
// in other time functions.
func (s State) NowDate() time.Time {
	return time.Now()
}

// Header gets the value of a header.
func (s State) Header() *dns.RR_Header {
	// TODO(miek)
	return nil
}

// IP gets the (remote) IP address of the client making the request.
func (s State) IP() string {
	ip, _, err := net.SplitHostPort(s.W.RemoteAddr().String())
	if err != nil {
		return s.W.RemoteAddr().String()
	}
	return ip
}

// Post gets the (remote) Port of the client making the request.
func (s State) Port() (string, error) {
	_, port, err := net.SplitHostPort(s.W.RemoteAddr().String())
	if err != nil {
		return "0", err
	}
	return port, nil
}

// Proto gets the protocol used as the transport. This
// will be udp or tcp.
func (s State) Proto() string {
	if _, ok := s.W.RemoteAddr().(*net.UDPAddr); ok {
		return "udp"
	}
	if _, ok := s.W.RemoteAddr().(*net.TCPAddr); ok {
		return "tcp"
	}
	return "udp"
}

// Family returns the family of the transport.
// 1 for IPv4 and 2 for IPv6.
func (s State) Family() int {
	var a net.IP
	ip := s.W.RemoteAddr()
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
func (s State) Do() bool {
	if o := s.Req.IsEdns0(); o != nil {
		return o.Do()
	}
	return false
}

// UDPSize returns if UDP buffer size advertised in the requests OPT record.
// Or when the request was over TCP, we return the maximum allowed size of 64K.
func (s State) Size() int {
	if s.Proto() == "tcp" {
		return dns.MaxMsgSize
	}
	if o := s.Req.IsEdns0(); o != nil {
		s := o.UDPSize()
		if s < dns.MinMsgSize {
			s = dns.MinMsgSize
		}
		return int(s)
	}
	return dns.MinMsgSize
}

// Type returns the type of the question as a string.
func (s State) Type() string {
	return dns.Type(s.Req.Question[0].Qtype).String()
}

// QType returns the type of the question as a uint16.
func (s State) QType() uint16 {
	return s.Req.Question[0].Qtype
}

// Name returns the name of the question in the request. Note
// this name will always have a closing dot and will be lower cased.
func (s State) Name() string {
	return strings.ToLower(dns.Name(s.Req.Question[0].Name).String())
}

// QName returns the name of the question in the request.
func (s State) QName() string {
	return dns.Name(s.Req.Question[0].Name).String()
}

// Class returns the class of the question in the request.
func (s State) Class() string {
	return dns.Class(s.Req.Question[0].Qclass).String()
}

// QClass returns the class of the question in the request.
func (s State) QClass() uint16 {
	return s.Req.Question[0].Qclass
}

// ErrorMessage returns an error message suitable for sending
// back to the client.
func (s State) ErrorMessage(rcode int) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(s.Req, rcode)
	return m
}

// AnswerMessage returns an error message suitable for sending
// back to the client.
func (s State) AnswerMessage() *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(s.Req)
	return m
}
