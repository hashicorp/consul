package rewrite

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func msgPrinter(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	w.WriteMsg(r)
	return 0, nil
}

func TestRewrite(t *testing.T) {
	rw := Rewrite{
		Next: middleware.HandlerFunc(msgPrinter),
		Rules: []Rule{
			NewSimpleRule("from.nl.", "to.nl."),
			NewSimpleRule("CH", "IN"),
			NewSimpleRule("ANY", "HINFO"),
		},
		noRevert: true,
	}

	tests := []struct {
		from  string
		fromT uint16
		fromC uint16
		to    string
		toT   uint16
		toC   uint16
	}{
		{"from.nl.", dns.TypeA, dns.ClassINET, "to.nl.", dns.TypeA, dns.ClassINET},
		{"a.nl.", dns.TypeA, dns.ClassINET, "a.nl.", dns.TypeA, dns.ClassINET},
		{"a.nl.", dns.TypeA, dns.ClassCHAOS, "a.nl.", dns.TypeA, dns.ClassINET},
		{"a.nl.", dns.TypeANY, dns.ClassINET, "a.nl.", dns.TypeHINFO, dns.ClassINET},
		// name is rewritten, type is not.
		{"from.nl.", dns.TypeANY, dns.ClassINET, "to.nl.", dns.TypeANY, dns.ClassINET},
		// name is not, type is, but class is, because class is the 2nd rule.
		{"a.nl.", dns.TypeANY, dns.ClassCHAOS, "a.nl.", dns.TypeANY, dns.ClassINET},
	}

	ctx := context.TODO()
	for i, tc := range tests {
		m := new(dns.Msg)
		m.SetQuestion(tc.from, tc.fromT)
		m.Question[0].Qclass = tc.fromC

		rec := dnsrecorder.New(&test.ResponseWriter{})
		rw.ServeDNS(ctx, rec, m)

		resp := rec.Msg
		if resp.Question[0].Name != tc.to {
			t.Errorf("Test %d: Expected Name to be '%s' but was '%s'", i, tc.to, resp.Question[0].Name)
		}
		if resp.Question[0].Qtype != tc.toT {
			t.Errorf("Test %d: Expected Type to be '%d' but was '%d'", i, tc.toT, resp.Question[0].Qtype)
		}
		if resp.Question[0].Qclass != tc.toC {
			t.Errorf("Test %d: Expected Class to be '%d' but was '%d'", i, tc.toC, resp.Question[0].Qclass)
		}
	}

	/*
		regexps := [][]string{
			{"/reg/", ".*", "/to", ""},
			{"/r/", "[a-z]+", "/toaz", "!.html|"},
			{"/url/", "a([a-z0-9]*)s([A-Z]{2})", "/to/{path}", ""},
			{"/ab/", "ab", "/ab?{query}", ".txt|"},
			{"/ab/", "ab", "/ab?type=html&{query}", ".html|"},
			{"/abc/", "ab", "/abc/{file}", ".html|"},
			{"/abcd/", "ab", "/a/{dir}/{file}", ".html|"},
			{"/abcde/", "ab", "/a#{fragment}", ".html|"},
			{"/ab/", `.*\.jpg`, "/ajpg", ""},
			{"/reggrp", `/ad/([0-9]+)([a-z]*)`, "/a{1}/{2}", ""},
			{"/reg2grp", `(.*)`, "/{1}", ""},
			{"/reg3grp", `(.*)/(.*)/(.*)`, "/{1}{2}{3}", ""},
		}

		for _, regexpRule := range regexps {
			var ext []string
			if s := strings.Split(regexpRule[3], "|"); len(s) > 1 {
				ext = s[:len(s)-1]
			}
			rule, err := NewComplexRule(regexpRule[0], regexpRule[1], regexpRule[2], 0, ext, nil)
			if err != nil {
				t.Fatal(err)
			}
			rw.Rules = append(rw.Rules, rule)
		}
	*/
	/*
		statusTests := []struct {
			status         int
			base           string
			to             string
			regexp         string
			statusExpected bool
		}{
			{400, "/status", "", "", true},
			{400, "/ignore", "", "", false},
			{400, "/", "", "^/ignore", false},
			{400, "/", "", "(.*)", true},
			{400, "/status", "", "", true},
		}

		for i, s := range statusTests {
			urlPath := fmt.Sprintf("/status%d", i)
			rule, err := NewComplexRule(s.base, s.regexp, s.to, s.status, nil, nil)
			if err != nil {
				t.Fatalf("Test %d: No error expected for rule but found %v", i, err)
			}
			rw.Rules = []Rule{rule}
			req, err := http.NewRequest("GET", urlPath, nil)
			if err != nil {
				t.Fatalf("Test %d: Could not create HTTP request: %v", i, err)
			}

			rec := httptest.NewRecorder()
			code, err := rw.ServeHTTP(rec, req)
			if err != nil {
				t.Fatalf("Test %d: No error expected for handler but found %v", i, err)
			}
			if s.statusExpected {
				if rec.Body.String() != "" {
					t.Errorf("Test %d: Expected empty body but found %s", i, rec.Body.String())
				}
				if code != s.status {
					t.Errorf("Test %d: Expected status code %d found %d", i, s.status, code)
				}
			} else {
				if code != 0 {
					t.Errorf("Test %d: Expected no status code found %d", i, code)
				}
			}
		}
	*/
}
