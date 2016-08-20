package test

import (
	"github.com/miekg/coredns/middleware"

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
