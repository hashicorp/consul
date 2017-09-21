package replacer

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestNewReplacer(t *testing.T) {
	w := dnstest.NewRecorder(&test.ResponseWriter{})

	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.MsgHdr.AuthenticatedData = true

	replaceValues := New(r, w, "")

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

	default:
		t.Fatal("Return Value from New Replacer expected pass type assertion into a replacer type\n")
	}
}

func TestSet(t *testing.T) {
	w := dnstest.NewRecorder(&test.ResponseWriter{})

	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeHINFO)
	r.MsgHdr.AuthenticatedData = true

	repl := New(r, w, "")

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
