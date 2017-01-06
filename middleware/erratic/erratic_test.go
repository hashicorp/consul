package erratic

import (
	"testing"

	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestErraticDrop(t *testing.T) {
	e := &Erratic{amount: 2} // 50% drops

	tests := []struct {
		expectedCode int
		expectedErr  error
		drop         bool
	}{
		{expectedCode: dns.RcodeSuccess, expectedErr: nil, drop: true},
		{expectedCode: dns.RcodeSuccess, expectedErr: nil, drop: false},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion("example.org.", dns.TypeA)

		rec := dnsrecorder.New(&test.ResponseWriter{})
		code, err := e.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error %q, but got %q", i, tc.expectedErr, err)
		}
		if code != int(tc.expectedCode) {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, tc.expectedCode, code)
		}

		if tc.drop && rec.Msg != nil {
			t.Errorf("Test %d: Expected dropped packet, but got %q", i, rec.Msg.Question[0].Name)
		}
	}
}
