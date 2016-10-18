// +build etcd

package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// uses some stuff from etcd_tests.go

func TestEtcdCacheAndDebug(t *testing.T) {
	corefile := `.:0 {
    etcd skydns.test {
        path /skydns
	debug
    }
    cache skydns.test
}`

	ex, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(ex, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}
	defer ex.Stop()

	etc := etcdMiddleware()
	log.SetOutput(ioutil.Discard)

	var ctx = context.TODO()
	for _, serv := range servicesCacheTest {
		set(ctx, t, etc, serv.Key, 0, serv)
		defer delete(ctx, t, etc, serv.Key)
	}

	p := proxy.New([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "b.example.skydns.test.", dns.TypeA)
	if err != nil {
		t.Errorf("Expected to receive reply, but didn't: %s", err)
	}
	checkResponse(t, resp)

	resp, err = p.Lookup(state, "o-o.debug.b.example.skydns.test.", dns.TypeA)
	if err != nil {
		t.Errorf("Expected to receive reply, but didn't: %s", err)
	}
	checkResponse(t, resp)
	if len(resp.Extra) != 1 {
		t.Errorf("Expected one RR in additional section, got: %d", len(resp.Extra))
	}

	resp, err = p.Lookup(state, "b.example.skydns.test.", dns.TypeA)
	if err != nil {
		t.Errorf("Expected to receive reply, but didn't: %s", err)
	}
	checkResponse(t, resp)
	if len(resp.Extra) != 0 {
		t.Errorf("Expected no RRs in additional section, got: %d", len(resp.Extra))
	}
}

func checkResponse(t *testing.T, resp *dns.Msg) {
	if len(resp.Answer) == 0 {
		t.Fatal("Expected to at least one RR in the answer section, got none")
	}
	if resp.Answer[0].Header().Rrtype != dns.TypeA {
		t.Errorf("Expected RR to A, got: %d", resp.Answer[0].Header().Rrtype)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.0.0.1" {
		t.Errorf("Expected 127.0.0.1, got: %d", resp.Answer[0].(*dns.A).A.String())
	}
}

var servicesCacheTest = []*msg.Service{
	{Host: "127.0.0.1", Port: 666, Key: "b.example.skydns.test."},
}
