package chaos

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestChaos(t *testing.T) {
	em := Chaos{
		Version: version,
		Authors: map[string]bool{"Miek Gieben": true},
	}

	tests := []struct {
		next          middleware.Handler
		qname         string
		qtype         uint16
		expectedCode  int
		expectedReply string
		expectedErr   error
	}{
		{
			next:          test.NextHandler(dns.RcodeSuccess, nil),
			qname:         "version.bind",
			expectedCode:  dns.RcodeSuccess,
			expectedReply: version,
			expectedErr:   nil,
		},
		{
			next:          test.NextHandler(dns.RcodeSuccess, nil),
			qname:         "authors.bind",
			expectedCode:  dns.RcodeSuccess,
			expectedReply: "Miek Gieben",
			expectedErr:   nil,
		},
		{
			next:         test.NextHandler(dns.RcodeSuccess, nil),
			qname:        "authors.bind",
			qtype:        dns.TypeSRV,
			expectedCode: dns.RcodeSuccess,
			expectedErr:  nil,
		},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		if tc.qtype == 0 {
			tc.qtype = dns.TypeTXT
		}
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)
		req.Question[0].Qclass = dns.ClassCHAOS
		em.Next = tc.next

		rec := dnsrecorder.New(&test.ResponseWriter{})
		code, err := em.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v", i, tc.expectedErr, err)
		}
		if code != int(tc.expectedCode) {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, tc.expectedCode, code)
		}
		if tc.expectedReply != "" {
			answer := rec.Msg.Answer[0].(*dns.TXT).Txt[0]
			if answer != tc.expectedReply {
				t.Errorf("Test %d: Expected answer %s, but got %s", i, tc.expectedReply, answer)
			}
		}
	}
}

const version = "CoreDNS-001"
