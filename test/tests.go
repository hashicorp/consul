package test

import (
	"testing"
	"time"

	"github.com/miekg/coredns/core"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/server"

	"github.com/miekg/dns"
)

func Msg(zone string, typ uint16, o *dns.OPT) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(zone, typ)
	if o != nil {
		m.Extra = []dns.RR{o}
	}
	return m
}

func Exchange(m *dns.Msg, server, net string) (*dns.Msg, error) {
	c := new(dns.Client)
	c.Net = net
	return middleware.Exchange(c, m, server)
}

// Server returns a test server and the tcp and udp listeners addresses.
func Server(t *testing.T, corefile string) (*server.Server, string, string, error) {
	srv, err := core.TestServer(t, corefile)
	if err != nil {
		return nil, "", "", err
	}
	go srv.ListenAndServe()

	time.Sleep(1 * time.Second) // yeah... I regret nothing
	tcp, udp := srv.LocalAddr()
	return srv, tcp.String(), udp.String(), nil
}
