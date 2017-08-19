package nonwriter

import (
	"testing"

	"github.com/miekg/dns"
)

func TestNonWriter(t *testing.T) {
	nw := New(nil)
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	if err := nw.WriteMsg(m); err != nil {
		t.Errorf("Got error when writing to nonwriter: %s", err)
	}
	if x := nw.Msg.Question[0].Name; x != "example.org." {
		t.Errorf("Expacted 'example.org.' got %q:", x)
	}
}
