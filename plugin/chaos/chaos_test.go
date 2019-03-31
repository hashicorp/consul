package chaos

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestChaos(t *testing.T) {
	em := Chaos{
		Version: version,
		Authors: []string{"Miek Gieben"},
	}

	tests := []struct {
		next          plugin.Handler
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

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
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
