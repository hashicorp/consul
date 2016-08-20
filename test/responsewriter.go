package test

import (
	"net"

	"github.com/miekg/dns"
)

type ResponseWriter struct{}

func (t *ResponseWriter) LocalAddr() net.Addr {
	ip := net.ParseIP("127.0.0.1")
	port := 53
	return &net.UDPAddr{IP: ip, Port: port, Zone: ""}
}

func (t *ResponseWriter) RemoteAddr() net.Addr {
	ip := net.ParseIP("10.240.0.1")
	port := 40212
	return &net.UDPAddr{IP: ip, Port: port, Zone: ""}
}

func (t *ResponseWriter) WriteMsg(m *dns.Msg) error     { return nil }
func (t *ResponseWriter) Write(buf []byte) (int, error) { return len(buf), nil }
func (t *ResponseWriter) Close() error                  { return nil }
func (t *ResponseWriter) TsigStatus() error             { return nil }
func (t *ResponseWriter) TsigTimersOnly(bool)           { return }
func (t *ResponseWriter) Hijack()                       { return }
