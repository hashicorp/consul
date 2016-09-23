package errors

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestErrors(t *testing.T) {
	buf := bytes.Buffer{}
	em := errorHandler{Log: log.New(&buf, "", 0)}

	testErr := errors.New("test error")
	tests := []struct {
		next         middleware.Handler
		expectedCode int
		expectedLog  string
		expectedErr  error
	}{
		{
			next:         genErrorHandler(dns.RcodeSuccess, nil),
			expectedCode: dns.RcodeSuccess,
			expectedLog:  "",
			expectedErr:  nil,
		},
		{
			next:         genErrorHandler(dns.RcodeNotAuth, testErr),
			expectedCode: dns.RcodeNotAuth,
			expectedLog:  fmt.Sprintf("[ERROR %d %s] %v\n", dns.RcodeNotAuth, "example.org. A", testErr),
			expectedErr:  testErr,
		},
	}

	ctx := context.TODO()
	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)

	for i, tc := range tests {
		em.Next = tc.next
		buf.Reset()
		rec := dnsrecorder.New(&test.ResponseWriter{})
		code, err := em.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v",
				i, tc.expectedErr, err)
		}
		if code != tc.expectedCode {
			t.Errorf("Test %d: Expected status code %d, but got %d",
				i, tc.expectedCode, code)
		}
		if log := buf.String(); !strings.Contains(log, tc.expectedLog) {
			t.Errorf("Test %d: Expected log %q, but got %q",
				i, tc.expectedLog, log)
		}
	}
}

func TestVisibleErrorWithPanic(t *testing.T) {
	const panicMsg = "I'm a panic"
	eh := errorHandler{
		Debug: true,
		Next: middleware.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
			panic(panicMsg)
		}),
	}

	ctx := context.TODO()
	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)

	rec := dnsrecorder.New(&test.ResponseWriter{})

	code, err := eh.ServeDNS(ctx, rec, req)
	if code != 0 {
		t.Errorf("Expected error handler to return 0 (it should write to response), got status %d", code)
	}
	if err != nil {
		t.Errorf("Expected error handler to return nil error (it should panic!), but got '%v'", err)
	}
}

func genErrorHandler(rcode int, err error) middleware.Handler {
	return middleware.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		return rcode, err
	})
}
