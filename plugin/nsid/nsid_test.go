package nsid

import (
	"encoding/hex"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/plugin/whoami"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestNsid(t *testing.T) {
	em := Nsid{
		Data: "NSID",
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
			next:          whoami.Whoami{},
			qname:         ".",
			expectedCode:  dns.RcodeSuccess,
			expectedReply: hex.EncodeToString([]byte("NSID")),
			expectedErr:   nil,
		},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		if tc.qtype == 0 {
			tc.qtype = dns.TypeA
		}
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)
		req.Question[0].Qclass = dns.ClassINET

		req.SetEdns0(4096, false)
		option := req.Extra[0].(*dns.OPT)
		option.Option = append(option.Option, &dns.EDNS0_NSID{Code: dns.EDNS0NSID, Nsid: ""})
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
			for _, extra := range rec.Msg.Extra {
				if option, ok := extra.(*dns.OPT); ok {
					e := option.Option[0].(*dns.EDNS0_NSID)
					if e.Nsid != tc.expectedReply {
						t.Errorf("Test %d: Expected answer %s, but got %s", i, tc.expectedReply, e.Nsid)
					}
				}
			}
		}
	}
}
