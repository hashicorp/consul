package replacer

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/metadata"
	"github.com/coredns/coredns/request"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestNewReplacer(t *testing.T) {
	w := dnstest.NewRecorder(&test.ResponseWriter{})

	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.MsgHdr.AuthenticatedData = true

	replaceValues := New(context.TODO(), r, w, "")

	switch v := replaceValues.(type) {
	case replacer:

		if v.replacements["{type}"] != "HINFO" {
			t.Errorf("Expected type to be HINFO, got %q", v.replacements["{type}"])
		}
		if v.replacements["{name}"] != "example.org." {
			t.Errorf("Expected request name to be example.org., got %q", v.replacements["{name}"])
		}
		if v.replacements["{size}"] != "29" { // size of request
			t.Errorf("Expected size to be 29, got %q", v.replacements["{size}"])
		}
		if !strings.Contains(v.replacements["{duration}"], "s") {
			t.Errorf("Expected units of time to be in seconds")
		}

	default:
		t.Fatal("Return Value from New Replacer expected pass type assertion into a replacer type\n")
	}
}

func TestSet(t *testing.T) {
	w := dnstest.NewRecorder(&test.ResponseWriter{})

	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.MsgHdr.AuthenticatedData = true

	repl := New(context.TODO(), r, w, "")

	repl.Set("name", "coredns.io.")
	repl.Set("type", "A")
	repl.Set("size", "20")

	if repl.Replace("This name is {name}") != "This name is coredns.io." {
		t.Error("Expected name replacement failed")
	}
	if repl.Replace("This type is {type}") != "This type is A" {
		t.Error("Expected type replacement failed")
	}
	if repl.Replace("The request size is {size}") != "The request size is 20" {
		t.Error("Expected size replacement failed")
	}
}

type testProvider map[string]metadata.Func

func (tp testProvider) Metadata(ctx context.Context, state request.Request) context.Context {
	for k, v := range tp {
		metadata.SetValueFunc(ctx, k, v)
	}
	return ctx
}

type testHandler struct{ ctx context.Context }

func (m *testHandler) Name() string { return "test" }

func (m *testHandler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	m.ctx = ctx
	return 0, nil
}

func TestMetadataReplacement(t *testing.T) {

	mdata := testProvider{
		"test/key2": func() string { return "two" },
	}

	tests := []struct {
		expr   string
		result string
	}{
		{"{name}", "example.org."},
		{"{/test/key2}", "two"},
		{"TYPE={type}, NAME={name}, BUFSIZE={>bufsize}, WHAT={/test/key2} .. and more", "TYPE=HINFO, NAME=example.org., BUFSIZE=512, WHAT=two .. and more"},
		{"{/test/key2}{/test/key4}", "two-"},
		{"{test/key2", "{test/key2"}, // if last } is missing, the end of format is considered static text
		{"{/test-key2}", "-"},        // everything that is not a placeholder for log or a metadata label is invalid
	}

	next := &testHandler{} // fake handler which stores the resulting context
	m := metadata.Metadata{
		Zones:     []string{"."},
		Providers: []metadata.Provider{mdata},
		Next:      next,
	}

	ctx := context.TODO()
	m.ServeDNS(ctx, &test.ResponseWriter{}, new(dns.Msg))
	nctx := next.ctx

	w := dnstest.NewRecorder(&test.ResponseWriter{})

	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.MsgHdr.AuthenticatedData = true

	repl := New(nctx, r, w, "-")

	for i, ts := range tests {
		r := repl.Replace(ts.expr)
		if r != ts.result {
			t.Errorf("Test %d - expr : %s, expected replacement being %s, and got %s", i, ts.expr, ts.result, r)
		}
	}
}
