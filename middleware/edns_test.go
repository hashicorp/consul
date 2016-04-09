package middleware

import (
	"testing"

	"github.com/miekg/dns"
)

func TestEdns0Version(t *testing.T) {
	m := ednsMsg()
	m.Extra[0].(*dns.OPT).SetVersion(2)

	_, err := Edns0Version(m)
	if err == nil {
		t.Errorf("expected wrong version, but got OK")
	}
}

func TestEdns0VersionNoEdns(t *testing.T) {
	m := ednsMsg()
	m.Extra = nil

	_, err := Edns0Version(m)
	if err != nil {
		t.Errorf("expected no error, but got one: %s", err)
	}
}

func ednsMsg() *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	m.Extra = append(m.Extra, o)
	return m
}
