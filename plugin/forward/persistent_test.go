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

	tr := newTransport(s.Addr, nil /* no TLS */)
	defer tr.Stop()

	c1, cache1, _ := tr.Dial("udp")
	c2, cache2, _ := tr.Dial("udp")
	c3, cache3, _ := tr.Dial("udp")

	if cache1 || cache2 || cache3 {
		t.Errorf("Expected non-cached connection")
	}

	tr.Yield(c1)
	tr.Yield(c2)
	tr.Yield(c3)
}
