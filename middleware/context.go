package middleware

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// This file contains the context and functions available for
// use in the templates.

// Context is the context with which Caddy templates are executed.
type Context struct {
	Root http.FileSystem // TODO(miek): needed
	Req  *dns.Msg
	W    dns.ResponseWriter
}

// Now returns the current timestamp in the specified format.
func (c Context) Now(format string) string {
	return time.Now().Format(format)
}

// NowDate returns the current date/time that can be used
// in other time functions.
func (c Context) NowDate() time.Time {
	return time.Now()
}

// Header gets the value of a header.
func (c Context) Header() *dns.RR_Header {
	// TODO(miek)
	return nil
}

// IP gets the (remote) IP address of the client making the request.
func (c Context) IP() string {
	ip, _, err := net.SplitHostPort(c.W.RemoteAddr().String())
	if err != nil {
		return c.W.RemoteAddr().String()
	}
	return ip
}

// Post gets the (remote) Port of the client making the request.
func (c Context) Port() (string, error) {
	_, port, err := net.SplitHostPort(c.W.RemoteAddr().String())
	if err != nil {
		return "0", err
	}
	return port, nil
}

// Proto gets the protocol used as the transport. This
// will be udp or tcp.
func (c Context) Proto() string {
	if _, ok := c.W.RemoteAddr().(*net.UDPAddr); ok {
		return "udp"
	}
	if _, ok := c.W.RemoteAddr().(*net.TCPAddr); ok {
		return "tcp"
	}
	return "udp"
}

// Family returns the family of the transport.
// 1 for IPv4 and 2 for IPv6.
func (c Context) Family() int {
	var a net.IP
	ip := c.W.RemoteAddr()
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

// Type returns the type of the question as a string.
func (c Context) Type() string {
	return dns.Type(c.Req.Question[0].Qtype).String()
}

// QType returns the type of the question as a uint16.
func (c Context) QType() uint16 {
	return c.Req.Question[0].Qtype
}

// Name returns the name of the question in the request. Note
// this name will always have a closing dot and will be lower cased.
func (c Context) Name() string {
	return strings.ToLower(dns.Name(c.Req.Question[0].Name).String())
}

// QName returns the name of the question in the request.
func (c Context) QName() string {
	return dns.Name(c.Req.Question[0].Name).String()
}

// Class returns the class of the question in the request.
func (c Context) Class() string {
	return dns.Class(c.Req.Question[0].Qclass).String()
}

// QClass returns the class of the question in the request.
func (c Context) QClass() uint16 {
	return c.Req.Question[0].Qclass
}

// More convience types for extracting stuff from a message?
// Header?

// ErrorMessage returns an error message suitable for sending
// back to the client.
func (c Context) ErrorMessage(rcode int) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(c.Req, rcode)
	return m
}

// AnswerMessage returns an error message suitable for sending
// back to the client.
func (c Context) AnswerMessage() *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(c.Req)
	return m
}
