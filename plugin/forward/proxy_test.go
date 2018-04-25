package forward

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/mholt/caddy"
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

		doneCnt := 0
		doneCh := make(chan bool)
		timeCh := time.After(10 * time.Second)
		go func() {
			p.connect(ctx, state, false, false)
			doneCh <- true
		}()
		go func() {
			p.connect(ctx, state, true, false)
			doneCh <- true
		}()
		go func() {
			p.close()
			doneCh <- true
		}()
		go func() {
			p.connect(ctx, state, false, false)
			doneCh <- true
		}()
		go func() {
			p.connect(ctx, state, true, false)
			doneCh <- true
		}()

		for doneCnt < 5 {
			select {
			case <-doneCh:
				doneCnt++
			case <-timeCh:
				t.Error("TestProxyClose is running too long, dumping goroutines:")
				buf := make([]byte, 100000)
				stackSize := runtime.Stack(buf, true)
				t.Fatal(string(buf[:stackSize]))
			}
		}
		if p.inProgress != 0 {
			t.Errorf("unexpected query in progress")
		}
		if p.state != stopped {
			t.Errorf("unexpected proxy state, expected %d, got %d", stopped, p.state)
		}
	}
}

func TestProxy(t *testing.T) {
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
		w.WriteMsg(ret)
	})
	defer s.Close()

	c := caddy.NewTestController("dns", "forward . "+s.Addr)
	f, err := parseForward(c)
	if err != nil {
		t.Errorf("Failed to create forwarder: %s", err)
	}
	f.OnStartup()
	defer f.OnShutdown()

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	if _, err := f.ServeDNS(context.TODO(), rec, m); err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}
	if x := rec.Msg.Answer[0].Header().Name; x != "example.org." {
		t.Errorf("Expected %s, got %s", "example.org.", x)
	}
}

func TestProxyTLSFail(t *testing.T) {
	// This is an udp/tcp test server, so we shouldn't reach it with TLS.
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
		w.WriteMsg(ret)
	})
	defer s.Close()

	c := caddy.NewTestController("dns", "forward . tls://"+s.Addr)
	f, err := parseForward(c)
	if err != nil {
		t.Errorf("Failed to create forwarder: %s", err)
	}
	f.OnStartup()
	defer f.OnShutdown()

	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	if _, err := f.ServeDNS(context.TODO(), rec, m); err == nil {
		t.Fatal("Expected *not* to receive reply, but got one")
	}
}
