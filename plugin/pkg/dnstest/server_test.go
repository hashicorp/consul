package dnstest

import (
	"testing"

	"github.com/miekg/dns"
)

func TestNewServer(t *testing.T) {
	s := NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		w.WriteMsg(ret)
	})
	defer s.Close()

	c := new(dns.Client)
	c.Net = "tcp"
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeSOA)
	ret, _, err := c.Exchange(m, s.Addr)
	if err != nil {
		t.Fatalf("Could not send message to dnstest.Server: %s", err)
	}
	if ret.Id != m.Id {
		t.Fatalf("Msg ID's should match, expected %d, got %d", m.Id, ret.Id)
	}
}
