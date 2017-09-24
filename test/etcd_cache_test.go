// +build etcd

package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// uses some stuff from etcd_tests.go

func TestEtcdCache(t *testing.T) {
	corefile := `.:0 {
    etcd skydns.test {
        path /skydns
    }
    cache skydns.test
}`

	ex, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer ex.Stop()

	etc := etcdPlugin()
	log.SetOutput(ioutil.Discard)

	var ctx = context.TODO()
	for _, serv := range servicesCacheTest {
		set(ctx, t, etc, serv.Key, 0, serv)
		defer delete(ctx, t, etc, serv.Key)
	}

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "b.example.skydns.test.", dns.TypeA)
	if err != nil {
		t.Errorf("Expected to receive reply, but didn't: %s", err)
	}
	checkResponse(t, resp)

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
		t.Errorf("Expected 127.0.0.1, got: %s", resp.Answer[0].(*dns.A).A.String())
	}
}

var servicesCacheTest = []*msg.Service{
	{Host: "127.0.0.1", Port: 666, Key: "b.example.skydns.test."},
}
