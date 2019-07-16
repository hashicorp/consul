package log

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/replacer"
	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func init() { clog.Discard() }

func TestLoggedStatus(t *testing.T) {
	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Class:     map[response.Class]struct{}{response.All: {}},
	}

	var f bytes.Buffer
	log.SetOutput(&f)

	logger := Logger{
		Rules: []Rule{rule},
		Next:  test.ErrorHandler(),
		repl:  replacer.New(),
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	rcode, _ := logger.ServeDNS(ctx, rec, r)
	if rcode != 2 {
		t.Errorf("Expected rcode to be 2 - was: %d", rcode)
	}

	logged := f.String()
	if !strings.Contains(logged, "A IN example.org. udp 29 false 512") {
		t.Errorf("Expected it to be logged. Logged string: %s", logged)
	}
}

func TestLoggedClassDenial(t *testing.T) {
	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Class:     map[response.Class]struct{}{response.Denial: {}},
	}

	var f bytes.Buffer
	log.SetOutput(&f)

	logger := Logger{
		Rules: []Rule{rule},
		Next:  test.ErrorHandler(),
		repl:  replacer.New(),
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	logger.ServeDNS(ctx, rec, r)

	logged := f.String()
	if len(logged) != 0 {
		t.Errorf("Expected it not to be logged, but got string: %s", logged)
	}
}

func TestLoggedClassError(t *testing.T) {
	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Class:     map[response.Class]struct{}{response.Error: {}},
	}

	var f bytes.Buffer
	log.SetOutput(&f)

	logger := Logger{
		Rules: []Rule{rule},
		Next:  test.ErrorHandler(),
		repl:  replacer.New(),
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	logger.ServeDNS(ctx, rec, r)

	logged := f.String()
	if !strings.Contains(logged, "SERVFAIL") {
		t.Errorf("Expected it to be logged. Logged string: %s", logged)
	}
}

func TestLogged(t *testing.T) {
	tests := []struct {
		Rules           []Rule
		Domain          string
		ShouldLog       bool
		ShouldString    string
		ShouldNOTString string // for test format
	}{
		// case for NameScope
		{
			Rules: []Rule{
				{
					NameScope: "example.org.",
					Format:    DefaultLogFormat,
					Class:     map[response.Class]struct{}{response.All: {}},
				},
			},
			Domain:       "example.org.",
			ShouldLog:    true,
			ShouldString: "A IN example.org.",
		},
		{
			Rules: []Rule{
				{
					NameScope: "example.org.",
					Format:    DefaultLogFormat,
					Class:     map[response.Class]struct{}{response.All: {}},
				},
			},
			Domain:       "example.net.",
			ShouldLog:    false,
			ShouldString: "",
		},
		{
			Rules: []Rule{
				{
					NameScope: "example.org.",
					Format:    DefaultLogFormat,
					Class:     map[response.Class]struct{}{response.All: {}},
				},
				{
					NameScope: "example.net.",
					Format:    DefaultLogFormat,
					Class:     map[response.Class]struct{}{response.All: {}},
				},
			},
			Domain:       "example.net.",
			ShouldLog:    true,
			ShouldString: "A IN example.net.",
		},

		// case for format
		{
			Rules: []Rule{
				{
					NameScope: ".",
					Format:    "{type} {class}",
					Class:     map[response.Class]struct{}{response.All: {}},
				},
			},
			Domain:          "example.org.",
			ShouldLog:       true,
			ShouldString:    "A IN",
			ShouldNOTString: "example.org",
		},
		{
			Rules: []Rule{
				{
					NameScope: ".",
					Format:    "{remote}:{port}",
					Class:     map[response.Class]struct{}{response.All: {}},
				},
			},
			Domain:          "example.org.",
			ShouldLog:       true,
			ShouldString:    "10.240.0.1:40212",
			ShouldNOTString: "A IN example.org",
		},
		{
			Rules: []Rule{
				{
					NameScope: ".",
					Format:    CombinedLogFormat,
					Class:     map[response.Class]struct{}{response.All: {}},
				},
			},
			Domain:       "example.org.",
			ShouldLog:    true,
			ShouldString: "\"0\"",
		},
	}

	for _, tc := range tests {
		var f bytes.Buffer
		log.SetOutput(&f)

		logger := Logger{
			Rules: tc.Rules,
			Next:  test.ErrorHandler(),
			repl:  replacer.New(),
		}

		ctx := context.TODO()
		r := new(dns.Msg)
		r.SetQuestion(tc.Domain, dns.TypeA)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := logger.ServeDNS(ctx, rec, r)
		if err != nil {
			t.Fatal(err)
		}

		logged := f.String()

		if !tc.ShouldLog && len(logged) != 0 {
			t.Errorf("Expected it not to be logged, but got string: %s", logged)
		}
		if tc.ShouldLog && !strings.Contains(logged, tc.ShouldString) {
			t.Errorf("Expected it to contains: %s. Logged string: %s", tc.ShouldString, logged)
		}
		if tc.ShouldLog && tc.ShouldNOTString != "" && strings.Contains(logged, tc.ShouldNOTString) {
			t.Errorf("Expected it to NOT contains: %s. Logged string: %s", tc.ShouldNOTString, logged)
		}
	}
}

func BenchmarkLogged(b *testing.B) {
	log.SetOutput(ioutil.Discard)

	rule := Rule{
		NameScope: ".",
		Format:    DefaultLogFormat,
		Class:     map[response.Class]struct{}{response.All: {}},
	}

	logger := Logger{
		Rules: []Rule{rule},
		Next:  test.ErrorHandler(),
		repl:  replacer.New(),
	}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		logger.ServeDNS(ctx, rec, r)
	}
}
