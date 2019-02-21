package replacer

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/metadata"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestReplacer(t *testing.T) {
	w := dnstest.NewRecorder(&test.ResponseWriter{})
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.MsgHdr.AuthenticatedData = true
	state := request.Request{W: w, Req: r}

	replacer := New()

	if x := replacer.Replace(context.TODO(), state, nil, "{type}"); x != "HINFO" {
		t.Errorf("Expected type to be HINFO, got %q", x)
	}
	if x := replacer.Replace(context.TODO(), state, nil, "{name}"); x != "example.org." {
		t.Errorf("Expected request name to be example.org., got %q", x)
	}
	if x := replacer.Replace(context.TODO(), state, nil, "{size}"); x != "29" {
		t.Errorf("Expected size to be 29, got %q", x)
	}
}

func TestLabels(t *testing.T) {
	w := dnstest.NewRecorder(&test.ResponseWriter{})
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.Id = 1053
	r.AuthenticatedData = true
	r.CheckingDisabled = true
	w.WriteMsg(r)
	state := request.Request{W: w, Req: r}

	replacer := New()
	ctx := context.TODO()

	// This couples the test very tightly to the code, but so be it.
	expect := map[string]string{
		"{type}":                    "HINFO",
		"{name}":                    "example.org.",
		"{class}":                   "IN",
		"{proto}":                   "udp",
		"{size}":                    "29",
		"{remote}":                  "10.240.0.1",
		"{port}":                    "40212",
		"{local}":                   "127.0.0.1",
		headerReplacer + "id}":      "1053",
		headerReplacer + "opcode}":  "0",
		headerReplacer + "do}":      "false",
		headerReplacer + "bufsize}": "512",
		"{rcode}":                   "NOERROR",
		"{rsize}":                   "29",
		"{duration}":                "0",
		headerReplacer + "rflags}":  "rd,ad,cd",
	}
	if len(expect) != len(labels) {
		t.Fatalf("Expect %d labels, got %d", len(expect), len(labels))
	}

	for _, lbl := range labels {
		repl := replacer.Replace(ctx, state, w, lbl)
		if lbl == "{duration}" {
			if repl[len(repl)-1] != 's' {
				t.Errorf("Expected seconds, got %q", repl)
			}
			continue
		}
		if repl != expect[lbl] {
			t.Errorf("Expected value %q, got %q", expect[lbl], repl)
		}
	}
}

func BenchmarkReplacer(b *testing.B) {
	w := dnstest.NewRecorder(&test.ResponseWriter{})
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.MsgHdr.AuthenticatedData = true
	state := request.Request{W: w, Req: r}

	b.ResetTimer()
	b.ReportAllocs()

	replacer := New()
	for i := 0; i < b.N; i++ {
		replacer.Replace(context.TODO(), state, nil, "{type} {name} {size}")
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
	tests := []struct {
		expr   string
		result string
	}{
		{"{/test/meta2}", "two"},
		{"{/test/meta2} {/test/key4}", "two -"},
		{"{/test/meta2} {/test/meta3}", "two three"},
	}

	next := &testHandler{}
	m := metadata.Metadata{
		Zones: []string{"."},
		Providers: []metadata.Provider{
			testProvider{"test/meta2": func() string { return "two" }},
			testProvider{"test/meta3": func() string { return "three" }},
		},
		Next: next,
	}

	m.ServeDNS(context.TODO(), &test.ResponseWriter{}, new(dns.Msg))
	ctx := next.ctx // important because the m.ServeDNS has only now populated the context

	w := dnstest.NewRecorder(&test.ResponseWriter{})
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)

	repl := New()
	state := request.Request{W: w, Req: r}

	for i, ts := range tests {
		r := repl.Replace(ctx, state, nil, ts.expr)
		if r != ts.result {
			t.Errorf("Test %d - expr : %s, expected %q, got %q", i, ts.expr, ts.result, r)
		}
	}
}

func TestMetadataMalformed(t *testing.T) {
	tests := []struct {
		expr   string
		result string
	}{
		{"{/test/meta2", "{/test/meta2"},
		{"{test/meta2} {/test/meta4}", "{test/meta2} -"},
		{"{test}", "{test}"},
	}

	next := &testHandler{}
	m := metadata.Metadata{
		Zones:     []string{"."},
		Providers: []metadata.Provider{testProvider{"test/meta2": func() string { return "two" }}},
		Next:      next,
	}

	m.ServeDNS(context.TODO(), &test.ResponseWriter{}, new(dns.Msg))
	ctx := next.ctx // important because the m.ServeDNS has only now populated the context

	w := dnstest.NewRecorder(&test.ResponseWriter{})
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)

	repl := New()
	state := request.Request{W: w, Req: r}

	for i, ts := range tests {
		r := repl.Replace(ctx, state, nil, ts.expr)
		if r != ts.result {
			t.Errorf("Test %d - expr : %s, expected %q, got %q", i, ts.expr, ts.result, r)
		}
	}
}
