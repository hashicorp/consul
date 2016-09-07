package test

import "github.com/miekg/dns"

func Msg(zone string, typ uint16, o *dns.OPT) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(zone, typ)
	if o != nil {
		m.Extra = []dns.RR{o}
	}
	return m
}
