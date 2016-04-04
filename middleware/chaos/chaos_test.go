package chaos

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	coretest "github.com/miekg/coredns/middleware/testing"

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
			next:          genHandler(dns.RcodeSuccess, nil),
			qname:         "version.bind",
			expectedCode:  dns.RcodeSuccess,
			expectedReply: version,
			expectedErr:   nil,
		},
		{
			next:          genHandler(dns.RcodeSuccess, nil),
			qname:         "authors.bind",
			expectedCode:  dns.RcodeSuccess,
			expectedReply: "Miek Gieben",
			expectedErr:   nil,
		},
		{
			next:         genHandler(dns.RcodeSuccess, nil),
			qname:        "authors.bind",
			qtype:        dns.TypeSRV,
			expectedCode: dns.RcodeSuccess,
			expectedErr:  nil,
		},
	}

	ctx := context.TODO()

	for i, test := range tests {
		req := new(dns.Msg)
		if test.qtype == 0 {
			test.qtype = dns.TypeTXT
		}
		req.SetQuestion(dns.Fqdn(test.qname), test.qtype)
		req.Question[0].Qclass = dns.ClassCHAOS
		em.Next = test.next

		rec := middleware.NewResponseRecorder(&coretest.ResponseWriter{})
		code, err := em.ServeDNS(ctx, rec, req)

		if err != test.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v", i, test.expectedErr, err)
		}
		if code != int(test.expectedCode) {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, test.expectedCode, code)
		}
		if test.expectedReply != "" {
			answer := rec.Msg().Answer[0].(*dns.TXT).Txt[0]
			if answer != test.expectedReply {
				t.Errorf("Test %d: Expected answer %s, but got %s", i, test.expectedReply, answer)
			}
		}
	}
}

func genHandler(rcode int, err error) middleware.Handler {
	return middleware.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		return rcode, err
	})
}

const version = "CoreDNS-001"
