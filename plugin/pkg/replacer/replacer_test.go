package replacer

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/metadata"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// This is the default format used by the log package
const CommonLogFormat = `{remote}:{port} - {>id} "{type} {class} {name} {proto} {size} {>do} {>bufsize}" {rcode} {>rflags} {rsize} {duration}`

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

func TestParseFormat(t *testing.T) {
	type formatTest struct {
		Format   string
		Expected replacer
	}
	tests := []formatTest{
		{
			Format:   "",
			Expected: replacer{},
		},
		{
			Format: "A",
			Expected: replacer{
				{"A", typeLiteral},
			},
		},
		{
			Format: "A {A}",
			Expected: replacer{
				{"A {A}", typeLiteral},
			},
		},
		{
			Format: "{{remote}}",
			Expected: replacer{
				{"{", typeLiteral},
				{"{remote}", typeLabel},
				{"}", typeLiteral},
			},
		},
		{
			Format: "{ A {remote} A }",
			Expected: replacer{
				{"{ A ", typeLiteral},
				{"{remote}", typeLabel},
				{" A }", typeLiteral},
			},
		},
		{
			Format: "{remote}}",
			Expected: replacer{
				{"{remote}", typeLabel},
				{"}", typeLiteral},
			},
		},
		{
			Format: "{{remote}",
			Expected: replacer{
				{"{", typeLiteral},
				{"{remote}", typeLabel},
			},
		},
		{
			Format: `Foo } {remote}`,
			Expected: replacer{
				// we don't do any optimizations to join adjacent literals
				{"Foo }", typeLiteral},
				{" ", typeLiteral},
				{"{remote}", typeLabel},
			},
		},
		{
			Format: `{ Foo`,
			Expected: replacer{
				{"{ Foo", typeLiteral},
			},
		},
		{
			Format: `} Foo`,
			Expected: replacer{
				{"}", typeLiteral},
				{" Foo", typeLiteral},
			},
		},
		{
			Format: "A { {remote} {type} {/meta1} } B",
			Expected: replacer{
				{"A { ", typeLiteral},
				{"{remote}", typeLabel},
				{" ", typeLiteral},
				{"{type}", typeLabel},
				{" ", typeLiteral},
				{"meta1", typeMetadata},
				{" }", typeLiteral},
				{" B", typeLiteral},
			},
		},
		{
			Format: `LOG {remote}:{port} - {>id} "{type} {class} {name} {proto} ` +
				`{size} {>do} {>bufsize}" {rcode} {>rflags} {rsize} {/meta1}-{/meta2} ` +
				`{duration} END OF LINE`,
			Expected: replacer{
				{"LOG ", typeLiteral},
				{"{remote}", typeLabel},
				{":", typeLiteral},
				{"{port}", typeLabel},
				{" - ", typeLiteral},
				{"{>id}", typeLabel},
				{` "`, typeLiteral},
				{"{type}", typeLabel},
				{" ", typeLiteral},
				{"{class}", typeLabel},
				{" ", typeLiteral},
				{"{name}", typeLabel},
				{" ", typeLiteral},
				{"{proto}", typeLabel},
				{" ", typeLiteral},
				{"{size}", typeLabel},
				{" ", typeLiteral},
				{"{>do}", typeLabel},
				{" ", typeLiteral},
				{"{>bufsize}", typeLabel},
				{`" `, typeLiteral},
				{"{rcode}", typeLabel},
				{" ", typeLiteral},
				{"{>rflags}", typeLabel},
				{" ", typeLiteral},
				{"{rsize}", typeLabel},
				{" ", typeLiteral},
				{"meta1", typeMetadata},
				{"-", typeLiteral},
				{"meta2", typeMetadata},
				{" ", typeLiteral},
				{"{duration}", typeLabel},
				{" END OF LINE", typeLiteral},
			},
		},
	}
	for i, x := range tests {
		r := parseFormat(x.Format)
		if !reflect.DeepEqual(r, x.Expected) {
			t.Errorf("%d: Expected:\n\t%+v\nGot:\n\t%+v", i, x.Expected, r)
		}
	}
}

func TestParseFormatNodes(t *testing.T) {
	// If we parse the format successfully the result of joining all the
	// segments should match the original format.
	formats := []string{
		"",
		"msg",
		"{remote}",
		"{remote}",
		"{{remote}",
		"{{remote}}",
		"{{remote}} A",
		CommonLogFormat,
		CommonLogFormat + " FOO} {BAR}",
		"A " + CommonLogFormat + " FOO} {BAR}",
		"A " + CommonLogFormat + " {/meta}",
	}
	join := func(r replacer) string {
		a := make([]string, len(r))
		for i, n := range r {
			if n.typ == typeMetadata {
				a[i] = "{/" + n.value + "}"
			} else {
				a[i] = n.value
			}
		}
		return strings.Join(a, "")
	}
	for _, format := range formats {
		r := parseFormat(format)
		s := join(r)
		if s != format {
			t.Errorf("Expected format to be: '%s' got: '%s'", format, s)
		}
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

	for lbl := range labels {
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

func BenchmarkReplacer_CommonLogFormat(b *testing.B) {

	w := dnstest.NewRecorder(&test.ResponseWriter{})
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.Id = 1053
	r.AuthenticatedData = true
	r.CheckingDisabled = true
	r.MsgHdr.AuthenticatedData = true
	w.WriteMsg(r)
	state := request.Request{W: w, Req: r}

	replacer := New()
	ctxt := context.TODO()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		replacer.Replace(ctxt, state, w, CommonLogFormat)
	}
}

func BenchmarkParseFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parseFormat(CommonLogFormat)
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
