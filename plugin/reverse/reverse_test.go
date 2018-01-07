package reverse

import (
	"net"
	"regexp"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestReverse(t *testing.T) {
	_, net4, _ := net.ParseCIDR("10.1.1.0/24")
	regexIP4, _ := regexp.Compile("^.*ip-" + regexMatchV4 + "\\.example\\.org\\.$")

	em := Reverse{
		Networks: networks{network{
			IPnet:        net4,
			Zone:         "example.org.",
			Template:     "ip-{ip}.example.org.",
			RegexMatchIP: regexIP4,
		}},
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
			qname:         "test.ip-10.1.1.2.example.org.",
			expectedCode:  dns.RcodeSuccess,
			expectedReply: "10.1.1.2",
			expectedErr:   nil,
		},
	}

	ctx := context.TODO()

	for i, tr := range tests {
		req := new(dns.Msg)

		tr.qtype = dns.TypeA
		req.SetQuestion(tr.qname, tr.qtype)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := em.ServeDNS(ctx, rec, req)

		if err != tr.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v", i, tr.expectedErr, err)
		}
		if code != int(tr.expectedCode) {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, tr.expectedCode, code)
		}
		if tr.expectedReply != "" {
			answer := rec.Msg.Answer[0].(*dns.A).A.String()
			if answer != tr.expectedReply {
				t.Errorf("Test %d: Expected answer %s, but got %s", i, tr.expectedReply, answer)
			}
		}
	}
}
