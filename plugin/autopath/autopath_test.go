package autopath

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

var autopathTestCases = []test.Case{
	{
		// search path expansion.
		Qname: "b.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.CNAME("b.example.org. 3600 IN CNAME b.com."),
			test.A("b.com." + defaultA),
		},
	},
	{
		// No search path expansion
		Qname: "a.example.com.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.A("a.example.com." + defaultA),
		},
	},
}

func newTestAutoPath() *AutoPath {
	ap := new(AutoPath)
	ap.Zones = []string{"."}
	ap.Next = nextHandler(map[string]int{
		"b.example.org.": dns.RcodeNameError,
		"b.com.":         dns.RcodeSuccess,
		"a.example.com.": dns.RcodeSuccess,
	})

	ap.search = []string{"example.org.", "example.com.", "com.", ""}
	return ap
}

func TestAutoPath(t *testing.T) {
	ap := newTestAutoPath()
	ctx := context.TODO()

	for _, tc := range autopathTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := ap.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			continue
		}

		// No sorting here as we want to check if the CNAME sits *before* the
		// test of the answer.
		resp := rec.Msg

		if err := test.Header(tc, resp); err != nil {
			t.Error(err)
			continue
		}
		if err := test.Section(tc, test.Answer, resp.Answer); err != nil {
			t.Error(err)
		}
		if err := test.Section(tc, test.Ns, resp.Ns); err != nil {
			t.Error(err)
		}
		if err := test.Section(tc, test.Extra, resp.Extra); err != nil {
			t.Error(err)
		}
	}
}

var autopathNoAnswerTestCases = []test.Case{
	{
		// search path expansion, no answer
		Qname: "c.example.org.", Qtype: dns.TypeA,
		Answer: []dns.RR{
			test.CNAME("b.example.org. 3600 IN CNAME b.com."),
			test.A("b.com." + defaultA),
		},
	},
}

func TestAutoPathNoAnswer(t *testing.T) {
	ap := newTestAutoPath()
	ctx := context.TODO()

	for _, tc := range autopathNoAnswerTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		rcode, err := ap.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			continue
		}
		if plugin.ClientWrite(rcode) {
			t.Fatalf("Expected no client write, got one for rcode %d", rcode)
		}
	}
}

// nextHandler returns a Handler that returns an answer for the question in the
// request per the domain->answer map. On success an RR will be returned: "qname 3600 IN A 127.0.0.53"
func nextHandler(mm map[string]int) test.Handler {
	return test.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		rcode, ok := mm[r.Question[0].Name]
		if !ok {
			return dns.RcodeServerFailure, nil
		}

		m := new(dns.Msg)
		m.SetReply(r)

		switch rcode {
		case dns.RcodeNameError:
			m.Rcode = rcode
			m.Ns = []dns.RR{soa}
			w.WriteMsg(m)
			return m.Rcode, nil

		case dns.RcodeSuccess:
			m.Rcode = rcode
			a, _ := dns.NewRR(r.Question[0].Name + defaultA)
			m.Answer = []dns.RR{a}

			w.WriteMsg(m)
			return m.Rcode, nil
		default:
			panic("nextHandler: unhandled rcode")
		}
	})
}

const defaultA = " 3600 IN A 127.0.0.53"

var soa = func() dns.RR {
	s, _ := dns.NewRR("example.org.		1800	IN	SOA	example.org. example.org. 1502165581 14400 3600 604800 14400")
	return s
}()

func TestInSearchPath(t *testing.T) {
	a := AutoPath{search: []string{"default.svc.cluster.local.", "svc.cluster.local.", "cluster.local."}}

	tests := []struct {
		qname string
		b     bool
	}{
		{"google.com", false},
		{"default.svc.cluster.local.", true},
		{"a.default.svc.cluster.local.", true},
		{"a.b.svc.cluster.local.", false},
	}
	for i, tc := range tests {
		got := firstInSearchPath(tc.qname, a.search)
		if got != tc.b {
			t.Errorf("Test %d, got %v, expected %v", i, got, tc.b)
		}
	}
}
