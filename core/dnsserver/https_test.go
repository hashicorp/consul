package dnsserver

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/miekg/dns"
)

func TestPostRequest(t *testing.T) {
	const ex = "example.org."

	m := new(dns.Msg)
	m.SetQuestion(ex, dns.TypeDNSKEY)

	out, _ := m.Pack()
	req, err := http.NewRequest(http.MethodPost, "https://"+ex+pathDOH+"?bla=foo:443", bytes.NewReader(out))
	if err != nil {
		t.Errorf("Failure to make request: %s", err)
	}
	req.Header.Set("content-type", mimeTypeDOH)
	req.Header.Set("accept", mimeTypeDOH)

	m, err = postRequestToMsg(req)
	if err != nil {
		t.Fatalf("Failure to get message from request: %s", err)
	}

	if x := m.Question[0].Name; x != ex {
		t.Errorf("Qname expected %s, got %s", ex, x)
	}
	if x := m.Question[0].Qtype; x != dns.TypeDNSKEY {
		t.Errorf("Qname expected %d, got %d", x, dns.TypeDNSKEY)
	}
}

func TestGetRequest(t *testing.T) {
	const ex = "example.org."

	m := new(dns.Msg)
	m.SetQuestion(ex, dns.TypeDNSKEY)

	out, _ := m.Pack()
	b64 := base64.RawURLEncoding.EncodeToString(out)

	req, err := http.NewRequest(http.MethodGet, "https://"+ex+pathDOH+"?dns="+b64, nil)
	if err != nil {
		t.Errorf("Failure to make request: %s", err)
	}
	req.Header.Set("content-type", mimeTypeDOH)
	req.Header.Set("accept", mimeTypeDOH)

	m, err = getRequestToMsg(req)
	if err != nil {
		t.Fatalf("Failure to get message from request: %s", err)
	}

	if x := m.Question[0].Name; x != ex {
		t.Errorf("Qname expected %s, got %s", ex, x)
	}
	if x := m.Question[0].Qtype; x != dns.TypeDNSKEY {
		t.Errorf("Qname expected %d, got %d", x, dns.TypeDNSKEY)
	}
}
