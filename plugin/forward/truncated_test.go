package forward

import (
	"sync/atomic"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestLookupTruncated(t *testing.T) {
	i := int32(0)
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		j := atomic.LoadInt32(&i)
		atomic.AddInt32(&i, 1)

		if j == 0 {
			ret := new(dns.Msg)
			ret.SetReply(r)
			ret.Truncated = true
			ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
			w.WriteMsg(ret)
			return

		}

		ret := new(dns.Msg)
		ret.SetReply(r)
		ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
		w.WriteMsg(ret)
	})
	defer s.Close()

	p := NewProxy(s.Addr, nil /* no TLS */)
	f := New()
	f.SetProxy(p)
	defer f.Close()

	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := f.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	// expect answer with TC
	if !resp.Truncated {
		t.Error("Expected to receive reply with TC bit set, but didn't")
	}

	resp, err = f.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	// expect answer without TC
	if resp.Truncated {
		t.Error("Expected to receive reply without TC bit set, but didn't")
	}
}

func TestForwardTruncated(t *testing.T) {
	i := int32(0)
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		j := atomic.LoadInt32(&i)
		atomic.AddInt32(&i, 1)

		if j == 0 {
			ret := new(dns.Msg)
			ret.SetReply(r)
			ret.Truncated = true
			ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
			w.WriteMsg(ret)
			return

		}

		ret := new(dns.Msg)
		ret.SetReply(r)
		ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
		w.WriteMsg(ret)
	})
	defer s.Close()

	f := New()

	p1 := NewProxy(s.Addr, nil /* no TLS */)
	f.SetProxy(p1)
	p2 := NewProxy(s.Addr, nil /* no TLS */)
	f.SetProxy(p2)
	defer f.Close()

	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	state.Req.SetQuestion("example.org.", dns.TypeA)
	resp, err := f.Forward(state)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}

	// expect answer with TC
	if !resp.Truncated {
		t.Error("Expected to receive reply with TC bit set, but didn't")
	}

	resp, err = f.Forward(state)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	// expect answer without TC
	if resp.Truncated {
		t.Error("Expected to receive reply without TC bit set, but didn't")
	}
}
