package rewrite

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/dnsrecorder"
	"github.com/coredns/coredns/middleware/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func msgPrinter(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	w.WriteMsg(r)
	return 0, nil
}

func TestNewRule(t *testing.T) {
	tests := []struct {
		args        []string
		shouldError bool
		expType     reflect.Type
	}{
		{[]string{}, true, nil},
		{[]string{"foo"}, true, nil},
		{[]string{"name"}, true, nil},
		{[]string{"name", "a.com"}, true, nil},
		{[]string{"name", "a.com", "b.com", "c.com"}, true, nil},
		{[]string{"name", "a.com", "b.com"}, false, reflect.TypeOf(&nameRule{})},
		{[]string{"type"}, true, nil},
		{[]string{"type", "a"}, true, nil},
		{[]string{"type", "any", "a", "a"}, true, nil},
		{[]string{"type", "any", "a"}, false, reflect.TypeOf(&typeRule{})},
		{[]string{"type", "XY", "WV"}, true, nil},
		{[]string{"type", "ANY", "WV"}, true, nil},
		{[]string{"class"}, true, nil},
		{[]string{"class", "IN"}, true, nil},
		{[]string{"class", "ch", "in", "in"}, true, nil},
		{[]string{"class", "ch", "in"}, false, reflect.TypeOf(&classRule{})},
		{[]string{"class", "XY", "WV"}, true, nil},
		{[]string{"class", "IN", "WV"}, true, nil},
		{[]string{"edns0"}, true, nil},
		{[]string{"edns0", "local"}, true, nil},
		{[]string{"edns0", "local", "set"}, true, nil},
		{[]string{"edns0", "local", "set", "0xffee"}, true, nil},
		{[]string{"edns0", "local", "set", "65518", "abcdefg"}, false, reflect.TypeOf(&edns0LocalRule{})},
		{[]string{"edns0", "local", "set", "0xffee", "abcdefg"}, false, reflect.TypeOf(&edns0LocalRule{})},
		{[]string{"edns0", "local", "append", "0xffee", "abcdefg"}, false, reflect.TypeOf(&edns0LocalRule{})},
		{[]string{"edns0", "local", "replace", "0xffee", "abcdefg"}, false, reflect.TypeOf(&edns0LocalRule{})},
		{[]string{"edns0", "local", "foo", "0xffee", "abcdefg"}, true, nil},
		{[]string{"edns0", "local", "set", "0xffee", "0xabcdefg"}, true, nil},
		{[]string{"edns0", "nsid", "set", "junk"}, true, nil},
		{[]string{"edns0", "nsid", "set"}, false, reflect.TypeOf(&edns0NsidRule{})},
		{[]string{"edns0", "nsid", "append"}, false, reflect.TypeOf(&edns0NsidRule{})},
		{[]string{"edns0", "nsid", "replace"}, false, reflect.TypeOf(&edns0NsidRule{})},
		{[]string{"edns0", "nsid", "foo"}, true, nil},
	}

	for i, tc := range tests {
		r, err := newRule(tc.args...)
		if err == nil && tc.shouldError {
			t.Errorf("Test %d: expected error but got success", i)
		} else if err != nil && !tc.shouldError {
			t.Errorf("Test %d: expected success but got error: %s", i, err)
		}

		if !tc.shouldError && reflect.TypeOf(r) != tc.expType {
			t.Errorf("Test %d: expected %q but got %q", i, tc.expType, r)
		}
	}
}

func TestRewrite(t *testing.T) {
	rules := []Rule{}
	r, _ := newNameRule("from.nl.", "to.nl.")
	rules = append(rules, r)
	r, _ = newClassRule("CH", "IN")
	rules = append(rules, r)
	r, _ = newTypeRule("ANY", "HINFO")
	rules = append(rules, r)

	rw := Rewrite{
		Next:     middleware.HandlerFunc(msgPrinter),
		Rules:    rules,
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
			t.Errorf("Test %d: Expected Name to be %q but was %q", i, tc.to, resp.Question[0].Name)
		}
		if resp.Question[0].Qtype != tc.toT {
			t.Errorf("Test %d: Expected Type to be '%d' but was '%d'", i, tc.toT, resp.Question[0].Qtype)
		}
		if resp.Question[0].Qclass != tc.toC {
			t.Errorf("Test %d: Expected Class to be '%d' but was '%d'", i, tc.toC, resp.Question[0].Qclass)
		}
	}
}

func TestRewriteEDNS0Local(t *testing.T) {
	rw := Rewrite{
		Next:     middleware.HandlerFunc(msgPrinter),
		noRevert: true,
	}

	tests := []struct {
		fromOpts []dns.EDNS0
		args     []string
		toOpts   []dns.EDNS0
	}{
		{
			[]dns.EDNS0{},
			[]string{"local", "set", "0xffee", "0xabcdef"},
			[]dns.EDNS0{&dns.EDNS0_LOCAL{Code: 0xffee, Data: []byte{0xab, 0xcd, 0xef}}},
		},
		{
			[]dns.EDNS0{},
			[]string{"local", "append", "0xffee", "abcdefghijklmnop"},
			[]dns.EDNS0{&dns.EDNS0_LOCAL{Code: 0xffee, Data: []byte("abcdefghijklmnop")}},
		},
		{
			[]dns.EDNS0{},
			[]string{"local", "replace", "0xffee", "abcdefghijklmnop"},
			[]dns.EDNS0{},
		},
		{
			[]dns.EDNS0{},
			[]string{"nsid", "set"},
			[]dns.EDNS0{&dns.EDNS0_NSID{Code: dns.EDNS0NSID, Nsid: ""}},
		},
		{
			[]dns.EDNS0{},
			[]string{"nsid", "append"},
			[]dns.EDNS0{&dns.EDNS0_NSID{Code: dns.EDNS0NSID, Nsid: ""}},
		},
		{
			[]dns.EDNS0{},
			[]string{"nsid", "replace"},
			[]dns.EDNS0{},
		},
	}

	ctx := context.TODO()
	for i, tc := range tests {
		m := new(dns.Msg)
		m.SetQuestion("example.com.", dns.TypeA)
		m.Question[0].Qclass = dns.ClassINET

		r, err := newEdns0Rule(tc.args...)
		if err != nil {
			t.Errorf("Error creating test rule: %s", err)
			continue
		}
		rw.Rules = []Rule{r}

		rec := dnsrecorder.New(&test.ResponseWriter{})
		rw.ServeDNS(ctx, rec, m)

		resp := rec.Msg
		o := resp.IsEdns0()
		if o == nil {
			t.Errorf("Test %d: EDNS0 options not set", i)
			continue
		}
		if !optsEqual(o.Option, tc.toOpts) {
			t.Errorf("Test %d: Expected %v but got %v", i, tc.toOpts, o)
		}
	}
}

func TestEdns0LocalMultiRule(t *testing.T) {
	rules := []Rule{}
	r, _ := newEdns0Rule("local", "replace", "0xffee", "abcdef")
	rules = append(rules, r)
	r, _ = newEdns0Rule("local", "set", "0xffee", "fedcba")
	rules = append(rules, r)

	rw := Rewrite{
		Next:     middleware.HandlerFunc(msgPrinter),
		Rules:    rules,
		noRevert: true,
	}

	tests := []struct {
		fromOpts []dns.EDNS0
		toOpts   []dns.EDNS0
	}{
		{
			nil,
			[]dns.EDNS0{&dns.EDNS0_LOCAL{Code: 0xffee, Data: []byte("fedcba")}},
		},
		{
			[]dns.EDNS0{&dns.EDNS0_LOCAL{Code: 0xffee, Data: []byte("foobar")}},
			[]dns.EDNS0{&dns.EDNS0_LOCAL{Code: 0xffee, Data: []byte("abcdef")}},
		},
	}

	ctx := context.TODO()
	for i, tc := range tests {
		m := new(dns.Msg)
		m.SetQuestion("example.com.", dns.TypeA)
		m.Question[0].Qclass = dns.ClassINET
		if tc.fromOpts != nil {
			o := m.IsEdns0()
			if o == nil {
				m.SetEdns0(4096, true)
				o = m.IsEdns0()
			}
			o.Option = append(o.Option, tc.fromOpts...)
		}
		rec := dnsrecorder.New(&test.ResponseWriter{})
		rw.ServeDNS(ctx, rec, m)

		resp := rec.Msg
		o := resp.IsEdns0()
		if o == nil {
			t.Errorf("Test %d: EDNS0 options not set", i)
			continue
		}
		if !optsEqual(o.Option, tc.toOpts) {
			t.Errorf("Test %d: Expected %v but got %v", i, tc.toOpts, o)
		}
	}
}

func optsEqual(a, b []dns.EDNS0) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		switch aa := a[i].(type) {
		case *dns.EDNS0_LOCAL:
			if bb, ok := b[i].(*dns.EDNS0_LOCAL); ok {
				if aa.Code != bb.Code {
					return false
				}
				if !bytes.Equal(aa.Data, bb.Data) {
					return false
				}
			} else {
				return false
			}
		case *dns.EDNS0_NSID:
			if bb, ok := b[i].(*dns.EDNS0_NSID); ok {
				if aa.Nsid != bb.Nsid {
					return false
				}
			} else {
				return false
			}
		default:
			return false
		}
	}
	return true
}
