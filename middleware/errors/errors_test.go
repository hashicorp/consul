package errors

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware"
	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestErrors(t *testing.T) {
	buf := bytes.Buffer{}
	em := ErrorHandler{Log: log.New(&buf, "", 0)}

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

	for i, test := range tests {
		em.Next = test.next
		buf.Reset()
		rec := middleware.NewResponseRecorder(&coretest.ResponseWriter{})
		code, err := em.ServeDNS(ctx, rec, req)

		if err != test.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v",
				i, test.expectedErr, err)
		}
		if code != test.expectedCode {
			t.Errorf("Test %d: Expected status code %d, but got %d",
				i, test.expectedCode, code)
		}
		if log := buf.String(); !strings.Contains(log, test.expectedLog) {
			t.Errorf("Test %d: Expected log %q, but got %q",
				i, test.expectedLog, log)
		}
	}
}

func TestVisibleErrorWithPanic(t *testing.T) {
	const panicMsg = "I'm a panic"
	eh := ErrorHandler{
		Debug: true,
		Next: middleware.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
			panic(panicMsg)
		}),
	}

	ctx := context.TODO()
	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)

	rec := middleware.NewResponseRecorder(&coretest.ResponseWriter{})

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
