package doh

import (
	"net/http"
	"testing"

	"github.com/miekg/dns"
)

func TestPostRequest(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeDNSKEY)

	req, err := NewRequest(http.MethodPost, "https://example.org:443", m)
	if err != nil {
		t.Errorf("Failure to make request: %s", err)
	}

	m, err = RequestToMsg(req)
	if err != nil {
		t.Fatalf("Failure to get message from request: %s", err)
	}

	if x := m.Question[0].Name; x != "example.org." {
		t.Errorf("Qname expected %s, got %s", "example.org.", x)
	}
	if x := m.Question[0].Qtype; x != dns.TypeDNSKEY {
		t.Errorf("Qname expected %d, got %d", x, dns.TypeDNSKEY)
	}
}

func TestGetRequest(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeDNSKEY)

	req, err := NewRequest(http.MethodGet, "https://example.org:443", m)
	if err != nil {
		t.Errorf("Failure to make request: %s", err)
	}

	m, err = RequestToMsg(req)
	if err != nil {
		t.Fatalf("Failure to get message from request: %s", err)
	}

	if x := m.Question[0].Name; x != "example.org." {
		t.Errorf("Qname expected %s, got %s", "example.org.", x)
	}
	if x := m.Question[0].Qtype; x != dns.TypeDNSKEY {
		t.Errorf("Qname expected %d, got %d", x, dns.TypeDNSKEY)
	}
}
