package template

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/coredns/coredns/plugin/test"

	gotmpl "text/template"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/miekg/dns"
)

func TestHandler(t *testing.T) {
	rcodeFallthrough := 3841 // reserved for private use, used to indicate a fallthrough
	exampleDomainATemplate := template{
		class:  dns.ClassINET,
		qtype:  dns.TypeA,
		regex:  []*regexp.Regexp{regexp.MustCompile("(^|[.])ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$")},
		answer: []*gotmpl.Template{gotmpl.Must(gotmpl.New("answer").Parse("{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"))},
	}
	exampleDomainANSTemplate := template{
		class:      dns.ClassINET,
		qtype:      dns.TypeA,
		regex:      []*regexp.Regexp{regexp.MustCompile("(^|[.])ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$")},
		answer:     []*gotmpl.Template{gotmpl.Must(gotmpl.New("answer").Parse("{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"))},
		additional: []*gotmpl.Template{gotmpl.Must(gotmpl.New("additional").Parse("ns0.example. IN A 203.0.113.8"))},
		authority:  []*gotmpl.Template{gotmpl.Must(gotmpl.New("authority").Parse("example. IN NS ns0.example.com."))},
	}
	exampleDomainMXTemplate := template{
		class:      dns.ClassINET,
		qtype:      dns.TypeMX,
		regex:      []*regexp.Regexp{regexp.MustCompile("(^|[.])ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$")},
		answer:     []*gotmpl.Template{gotmpl.Must(gotmpl.New("answer").Parse("{{ .Name }} 60 MX 10 {{ .Name }}"))},
		additional: []*gotmpl.Template{gotmpl.Must(gotmpl.New("additional").Parse("{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"))},
	}
	invalidDomainTemplate := template{
		class:  dns.ClassANY,
		qtype:  dns.TypeANY,
		regex:  []*regexp.Regexp{regexp.MustCompile("[.]invalid[.]$")},
		rcode:  dns.RcodeNameError,
		answer: []*gotmpl.Template{gotmpl.Must(gotmpl.New("answer").Parse("invalid. 60 {{ .Class }} SOA a.invalid. b.invalid. (1 60 60 60 60)"))},
	}
	rcodeServfailTemplate := template{
		class: dns.ClassANY,
		qtype: dns.TypeANY,
		regex: []*regexp.Regexp{regexp.MustCompile(".*")},
		rcode: dns.RcodeServerFailure,
	}
	brokenTemplate := template{
		class:  dns.ClassINET,
		qtype:  dns.TypeA,
		regex:  []*regexp.Regexp{regexp.MustCompile("[.]example[.]$")},
		answer: []*gotmpl.Template{gotmpl.Must(gotmpl.New("answer").Parse("{{ .Name }} 60 IN TXT \"{{ index .Match 2 }}\""))},
	}
	nonRRTemplate := template{
		class:  dns.ClassINET,
		qtype:  dns.TypeA,
		regex:  []*regexp.Regexp{regexp.MustCompile("[.]example[.]$")},
		answer: []*gotmpl.Template{gotmpl.Must(gotmpl.New("answer").Parse("{{ .Name }}"))},
	}
	nonRRAdditionalTemplate := template{
		class:      dns.ClassINET,
		qtype:      dns.TypeA,
		regex:      []*regexp.Regexp{regexp.MustCompile("[.]example[.]$")},
		additional: []*gotmpl.Template{gotmpl.Must(gotmpl.New("answer").Parse("{{ .Name }}"))},
	}
	nonRRAuthoritativeTemplate := template{
		class:     dns.ClassINET,
		qtype:     dns.TypeA,
		regex:     []*regexp.Regexp{regexp.MustCompile("[.]example[.]$")},
		authority: []*gotmpl.Template{gotmpl.Must(gotmpl.New("authority").Parse("{{ .Name }}"))},
	}

	tests := []struct {
		tmpl           template
		qname          string
		qclass         uint16
		qtype          uint16
		name           string
		expectedCode   int
		expectedErr    string
		verifyResponse func(*dns.Msg) error
	}{
		{
			name:         "RcodeServFail",
			tmpl:         rcodeServfailTemplate,
			qclass:       dns.ClassANY,
			qtype:        dns.TypeANY,
			qname:        "test.invalid.",
			expectedCode: dns.RcodeServerFailure,
			verifyResponse: func(r *dns.Msg) error {
				return nil
			},
		},
		{
			name:         "ExampleDomainNameMismatch",
			tmpl:         exampleDomainATemplate,
			qclass:       dns.ClassINET,
			qtype:        dns.TypeA,
			qname:        "test.invalid.",
			expectedCode: rcodeFallthrough,
		},
		{
			name:         "BrokenTemplate",
			tmpl:         brokenTemplate,
			qclass:       dns.ClassINET,
			qtype:        dns.TypeANY,
			qname:        "test.example.",
			expectedCode: dns.RcodeServerFailure,
			expectedErr:  `template: answer:1:26: executing "answer" at <index .Match 2>: error calling index: index out of range: 2`,
			verifyResponse: func(r *dns.Msg) error {
				return nil
			},
		},
		{
			name:         "NonRRTemplate",
			tmpl:         nonRRTemplate,
			qclass:       dns.ClassINET,
			qtype:        dns.TypeANY,
			qname:        "test.example.",
			expectedCode: dns.RcodeServerFailure,
			expectedErr:  `dns: not a TTL: "test.example." at line: 1:13`,
			verifyResponse: func(r *dns.Msg) error {
				return nil
			},
		},
		{
			name:         "NonRRAdditionalTemplate",
			tmpl:         nonRRAdditionalTemplate,
			qclass:       dns.ClassINET,
			qtype:        dns.TypeANY,
			qname:        "test.example.",
			expectedCode: dns.RcodeServerFailure,
			expectedErr:  `dns: not a TTL: "test.example." at line: 1:13`,
			verifyResponse: func(r *dns.Msg) error {
				return nil
			},
		},
		{
			name:         "NonRRAuthorityTemplate",
			tmpl:         nonRRAuthoritativeTemplate,
			qclass:       dns.ClassINET,
			qtype:        dns.TypeANY,
			qname:        "test.example.",
			expectedCode: dns.RcodeServerFailure,
			expectedErr:  `dns: not a TTL: "test.example." at line: 1:13`,
			verifyResponse: func(r *dns.Msg) error {
				return nil
			},
		},
		{
			name:   "ExampleDomainMatch",
			tmpl:   exampleDomainATemplate,
			qclass: dns.ClassINET,
			qtype:  dns.TypeA,
			qname:  "ip-10-95-12-8.example.",
			verifyResponse: func(r *dns.Msg) error {
				if len(r.Answer) != 1 {
					return fmt.Errorf("expected 1 answer, got %v", len(r.Answer))
				}
				if r.Answer[0].Header().Rrtype != dns.TypeA {
					return fmt.Errorf("expected an A record anwser, got %v", dns.TypeToString[r.Answer[0].Header().Rrtype])
				}
				if r.Answer[0].(*dns.A).A.String() != "10.95.12.8" {
					return fmt.Errorf("expected an A record for 10.95.12.8, got %v", r.Answer[0].String())
				}
				return nil
			},
		},
		{
			name:   "ExampleDomainMXMatch",
			tmpl:   exampleDomainMXTemplate,
			qclass: dns.ClassINET,
			qtype:  dns.TypeMX,
			qname:  "ip-10-95-12-8.example.",
			verifyResponse: func(r *dns.Msg) error {
				if len(r.Answer) != 1 {
					return fmt.Errorf("expected 1 answer, got %v", len(r.Answer))
				}
				if r.Answer[0].Header().Rrtype != dns.TypeMX {
					return fmt.Errorf("expected an A record anwser, got %v", dns.TypeToString[r.Answer[0].Header().Rrtype])
				}
				if len(r.Extra) != 1 {
					return fmt.Errorf("expected 1 extra record, got %v", len(r.Extra))
				}
				if r.Extra[0].Header().Rrtype != dns.TypeA {
					return fmt.Errorf("expected an additional A record, got %v", dns.TypeToString[r.Extra[0].Header().Rrtype])
				}
				return nil
			},
		},
		{
			name:   "ExampleDomainANSMatch",
			tmpl:   exampleDomainANSTemplate,
			qclass: dns.ClassINET,
			qtype:  dns.TypeA,
			qname:  "ip-10-95-12-8.example.",
			verifyResponse: func(r *dns.Msg) error {
				if len(r.Answer) != 1 {
					return fmt.Errorf("expected 1 answer, got %v", len(r.Answer))
				}
				if r.Answer[0].Header().Rrtype != dns.TypeA {
					return fmt.Errorf("expected an A record anwser, got %v", dns.TypeToString[r.Answer[0].Header().Rrtype])
				}
				if len(r.Extra) != 1 {
					return fmt.Errorf("expected 1 extra record, got %v", len(r.Extra))
				}
				if r.Extra[0].Header().Rrtype != dns.TypeA {
					return fmt.Errorf("expected an additional A record, got %v", dns.TypeToString[r.Extra[0].Header().Rrtype])
				}
				if len(r.Ns) != 1 {
					return fmt.Errorf("expected 1 authoritative record, got %v", len(r.Extra))
				}
				if r.Ns[0].Header().Rrtype != dns.TypeNS {
					return fmt.Errorf("expected an authoritative NS record, got %v", dns.TypeToString[r.Extra[0].Header().Rrtype])
				}
				return nil
			},
		},
		{
			name:         "ExampleDomainMismatchType",
			tmpl:         exampleDomainATemplate,
			qclass:       dns.ClassINET,
			qtype:        dns.TypeMX,
			qname:        "ip-10-95-12-8.example.",
			expectedCode: rcodeFallthrough,
		},
		{
			name:         "ExampleDomainMismatchClass",
			tmpl:         exampleDomainATemplate,
			qclass:       dns.ClassCHAOS,
			qtype:        dns.TypeA,
			qname:        "ip-10-95-12-8.example.",
			expectedCode: rcodeFallthrough,
		},
		{
			name:         "ExampleInvalidNXDOMAIN",
			tmpl:         invalidDomainTemplate,
			qclass:       dns.ClassINET,
			qtype:        dns.TypeMX,
			qname:        "test.invalid.",
			expectedCode: dns.RcodeNameError,
			verifyResponse: func(r *dns.Msg) error {
				if len(r.Answer) != 1 {
					return fmt.Errorf("expected 1 answer, got %v", len(r.Answer))
				}
				if r.Answer[0].Header().Rrtype != dns.TypeSOA {
					return fmt.Errorf("expected an SOA record anwser, got %v", dns.TypeToString[r.Answer[0].Header().Rrtype])
				}
				return nil
			},
		},
	}

	ctx := context.TODO()

	for _, tr := range tests {
		handler := Handler{
			Next:      test.NextHandler(rcodeFallthrough, nil),
			Templates: []template{tr.tmpl},
		}
		req := &dns.Msg{
			Question: []dns.Question{{
				Name:   tr.qname,
				Qclass: tr.qclass,
				Qtype:  tr.qtype,
			}},
		}
		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := handler.ServeDNS(ctx, rec, req)
		if err == nil && tr.expectedErr != "" {
			t.Errorf("Test %v expected error: %v, got nothing", tr.name, tr.expectedErr)
		}
		if err != nil && tr.expectedErr == "" {
			t.Errorf("Test %v expected no error got: %v", tr.name, err)
		}
		if err != nil && tr.expectedErr != "" && err.Error() != tr.expectedErr {
			t.Errorf("Test %v expected error: %v, got: %v", tr.name, tr.expectedErr, err)
		}
		if code != tr.expectedCode {
			t.Errorf("Test %v expected response code %v, got %v", tr.name, tr.expectedCode, code)
		}
		if err == nil && code != rcodeFallthrough {
			// only verify if we got no error and expected no error
			if err := tr.verifyResponse(rec.Msg); err != nil {
				t.Errorf("Test %v could not verify the response: %v", tr.name, err)
			}
		}
	}
}
