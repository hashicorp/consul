package forward

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"

	"github.com/miekg/dns"
)

func TestPersistent(t *testing.T) {
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		w.WriteMsg(ret)
	})
	defer s.Close()

	h := newHost(s.Addr)
	tr := newTransport(h)
	defer tr.Stop()

	c1, _ := tr.Dial("udp")
	c2, _ := tr.Dial("udp")
	c3, _ := tr.Dial("udp")

	tr.Yield(c1)
	tr.Yield(c2)
	tr.Yield(c3)

	if x := tr.Len(); x != 3 {
		t.Errorf("Expected cache size to be 3, got %d", x)
	}

	tr.Dial("udp")
	if x := tr.Len(); x != 2 {
		t.Errorf("Expected cache size to be 2, got %d", x)
	}

	tr.Dial("udp")
	if x := tr.Len(); x != 1 {
		t.Errorf("Expected cache size to be 2, got %d", x)
	}
}
