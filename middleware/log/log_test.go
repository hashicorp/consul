package log

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/pkg/response"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestLoggedStatus(t *testing.T) {
	var f bytes.Buffer
	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Log:       log.New(&f, "", 0),
	}

	logger := Logger{
		Rules: []Rule{rule},
		Next:  test.ErrorHandler(),
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnsrecorder.New(&test.ResponseWriter{})

	rcode, _ := logger.ServeDNS(ctx, rec, r)
	if rcode != 0 {
		t.Errorf("Expected rcode to be 0 - was: %d", rcode)
	}

	logged := f.String()
	if !strings.Contains(logged, "A IN example.org. udp false 512") {
		t.Errorf("Expected it to be logged. Logged string: %s", logged)
	}
}

func TestLoggedClassDenial(t *testing.T) {
	var f bytes.Buffer
	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Log:       log.New(&f, "", 0),
		Class:     response.Denial,
	}

	logger := Logger{
		Rules: []Rule{rule},
		Next:  test.ErrorHandler(),
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnsrecorder.New(&test.ResponseWriter{})

	logger.ServeDNS(ctx, rec, r)

	logged := f.String()
	if len(logged) != 0 {
		t.Errorf("Expected it not to be logged, but got string: %s", logged)
	}
}

func TestLoggedClassError(t *testing.T) {
	var f bytes.Buffer
	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Log:       log.New(&f, "", 0),
		Class:     response.Error,
	}

	logger := Logger{
		Rules: []Rule{rule},
		Next:  test.ErrorHandler(),
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnsrecorder.New(&test.ResponseWriter{})

	logger.ServeDNS(ctx, rec, r)

	logged := f.String()
	if !strings.Contains(logged, "SERVFAIL") {
		t.Errorf("Expected it to be logged. Logged string: %s", logged)
	}
}
