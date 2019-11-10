package bufsize

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/plugin/whoami"

	"github.com/miekg/dns"
)

func TestBufsize(t *testing.T) {
	em := Bufsize{
		Size: 512,
	}

	tests := []struct {
		next            plugin.Handler
		qname           string
		inputBufsize    uint16
		outgoingBufsize uint16
		expectedErr     error
	}{
		// This plugin is responsible for limiting outgoing query's bufize
		{
			next:            whoami.Whoami{},
			qname:           ".",
			inputBufsize:    1200,
			outgoingBufsize: 512,
			expectedErr:     nil,
		},
		// If EDNS is not enabled, this plugin adds it
		{
			next:            whoami.Whoami{},
			qname:           ".",
			outgoingBufsize: 512,
			expectedErr:     nil,
		},
	}

	for i, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(tc.qname), dns.TypeA)
		req.Question[0].Qclass = dns.ClassINET
		em.Next = tc.next

		if tc.inputBufsize != 0 {
			req.SetEdns0(tc.inputBufsize, false)
		}

		_, err := em.ServeDNS(context.Background(), &test.ResponseWriter{}, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error is %v, but got %v", i, tc.expectedErr, err)
		}

		if tc.outgoingBufsize != 0 {
			for _, extra := range req.Extra {
				if option, ok := extra.(*dns.OPT); ok {
					b := option.UDPSize()
					if b != tc.outgoingBufsize {
						t.Errorf("Test %d: Expected outgoing bufsize is %d, but got %d", i, tc.outgoingBufsize, b)
					}
				} else {
					t.Errorf("Test %d: Not found OPT RR.", i)
				}
			}
		}
	}
}
