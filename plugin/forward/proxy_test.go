package forward

import (
	"context"
	"sync"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestProxyClose(t *testing.T) {
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		w.WriteMsg(ret)
	})
	defer s.Close()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)
	state := request.Request{W: &test.ResponseWriter{}, Req: req}
	ctx := context.TODO()

	for i := 0; i < 100; i++ {
		p := NewProxy(s.Addr, nil /* no TLS */)
		p.start(hcDuration)

		var wg sync.WaitGroup
		wg.Add(5)
		go func() {
			p.connect(ctx, state, false, false)
			wg.Done()
		}()
		go func() {
			p.connect(ctx, state, true, false)
			wg.Done()
		}()
		go func() {
			p.close()
			wg.Done()
		}()
		go func() {
			p.connect(ctx, state, false, false)
			wg.Done()
		}()
		go func() {
			p.connect(ctx, state, true, false)
			wg.Done()
		}()
		wg.Wait()

		if p.inProgress != 0 {
			t.Errorf("unexpected query in progress")
		}
		if p.state != stopped {
			t.Errorf("unexpected proxy state, expected %d, got %d", stopped, p.state)
		}
	}
}
