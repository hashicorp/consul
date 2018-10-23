package erratic

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestErraticDrop(t *testing.T) {
	e := &Erratic{drop: 2} // 50% drops

	tests := []struct {
		rrtype       uint16
		expectedCode int
		expectedErr  error
		drop         bool
	}{
		{rrtype: dns.TypeA, expectedCode: dns.RcodeSuccess, expectedErr: nil, drop: true},
		{rrtype: dns.TypeA, expectedCode: dns.RcodeSuccess, expectedErr: nil, drop: false},
		{rrtype: dns.TypeAAAA, expectedCode: dns.RcodeSuccess, expectedErr: nil, drop: true},
		{rrtype: dns.TypeHINFO, expectedCode: dns.RcodeServerFailure, expectedErr: nil, drop: false},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion("example.org.", tc.rrtype)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := e.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error %q, but got %q", i, tc.expectedErr, err)
		}
		if code != int(tc.expectedCode) {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, tc.expectedCode, code)
		}

		if tc.drop && rec.Msg != nil {
			t.Errorf("Test %d: Expected dropped message, but got %q", i, rec.Msg.Question[0].Name)
		}
	}
}

func TestErraticTruncate(t *testing.T) {
	e := &Erratic{truncate: 2} // 50% drops

	tests := []struct {
		expectedCode int
		expectedErr  error
		truncate     bool
	}{
		{expectedCode: dns.RcodeSuccess, expectedErr: nil, truncate: true},
		{expectedCode: dns.RcodeSuccess, expectedErr: nil, truncate: false},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion("example.org.", dns.TypeA)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := e.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error %q, but got %q", i, tc.expectedErr, err)
		}
		if code != int(tc.expectedCode) {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, tc.expectedCode, code)
		}

		if tc.truncate && !rec.Msg.Truncated {
			t.Errorf("Test %d: Expected truncated message, but got %q", i, rec.Msg.Question[0].Name)
		}
	}
}

func TestAxfr(t *testing.T) {
	e := &Erratic{truncate: 0} // nothing, just check if we can get an axfr

	ctx := context.TODO()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeAXFR)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := e.ServeDNS(ctx, rec, req)
	if err != nil {
		t.Errorf("Failed to set up AXFR: %s", err)
	}
	if x := rec.Msg.Answer[0].Header().Rrtype; x != dns.TypeSOA {
		t.Errorf("Expected for record to be %d, got %d", dns.TypeSOA, x)
	}
}

func TestErratic(t *testing.T) {
	e := &Erratic{drop: 0, delay: 0}

	ctx := context.TODO()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	e.ServeDNS(ctx, rec, req)

	if rec.Msg.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected A response, got %d type", rec.Msg.Answer[0].Header().Rrtype)
	}
}
