package log

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware"
	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type erroringMiddleware struct{}

func (erroringMiddleware) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return dns.RcodeServerFailure, nil
}

func TestLoggedStatus(t *testing.T) {
	var f bytes.Buffer
	var next erroringMiddleware
	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Log:       log.New(&f, "", 0),
	}

	logger := Logger{
		Rules: []Rule{rule},
		Next:  next,
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := middleware.NewResponseRecorder(&coretest.ResponseWriter{})

	rcode, _ := logger.ServeDNS(ctx, rec, r)
	if rcode != 0 {
		t.Error("Expected rcode to be 0 - was", rcode)
	}

	logged := f.String()
	if !strings.Contains(logged, "A IN example.org. udp false 512") {
		t.Error("Expected it to be logged. Logged string -", logged)
	}
}
