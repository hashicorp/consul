// +build etcd

package etcd

import (
	"net"
	"sort"
	"strconv"
	"testing"

	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/dnsrecorder"
	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

func fakeStubServerExampleNet(t *testing.T) (*dns.Server, string) {
	server, addr, err := test.UDPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create a UDP server: %s", err)
	}
	// add handler for example.net
	dns.HandleFunc("example.net.", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = []dns.RR{test.A("example.net.	86400	IN	A	93.184.216.34")}
		w.WriteMsg(m)
	})

	return server, addr
}

func TestStubLookup(t *testing.T) {
	server, addr := fakeStubServerExampleNet(t)
	defer server.Shutdown()

	host, p, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(p)
	exampleNetStub := &msg.Service{Host: host, Port: port, Key: "a.example.net.stub.dns.skydns.test."}
	servicesStub = append(servicesStub, exampleNetStub)

	etc := newEtcdMiddleware()

	for _, serv := range servicesStub {
		set(t, etc, serv.Key, 0, serv)
		defer delete(t, etc, serv.Key)
	}

	etc.updateStubZones()

	for _, tc := range dnsTestCasesStub {
		m := tc.Msg()

		rec := dnsrecorder.New(&test.ResponseWriter{})
		_, err := etc.ServeDNS(ctxt, rec, m)
		if err != nil && m.Question[0].Name == "example.org." {
			// This is OK, we expect this backend to *not* work.
			continue
		}
		if err != nil {
			t.Errorf("expected no error, got %v for %s\n", err, m.Question[0].Name)
		}
		resp := rec.Msg
		if resp == nil {
			// etcd not running?
			continue
		}
		sort.Sort(test.RRSet(resp.Answer))
		sort.Sort(test.RRSet(resp.Ns))
		sort.Sort(test.RRSet(resp.Extra))

		if !test.Header(t, tc, resp) {
			t.Logf("%v\n", resp)
			continue
		}
		if !test.Section(t, tc, test.Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !test.Section(t, tc, test.Ns, resp.Ns) {
			t.Logf("%v\n", resp)
		}
		if !test.Section(t, tc, test.Extra, resp.Extra) {
			t.Logf("%v\n", resp)
		}
	}
}

var servicesStub = []*msg.Service{
	// Two tests, ask a question that should return servfail because remote it no accessible
	// and one with edns0 option added, that should return refused.
	{Host: "127.0.0.1", Port: 666, Key: "b.example.org.stub.dns.skydns.test."},
}

var dnsTestCasesStub = []test.Case{
	{
		Qname: "example.org.", Qtype: dns.TypeA, Rcode: dns.RcodeServerFailure,
	},
	{
		Qname: "example.net.", Qtype: dns.TypeA,
		Answer: []dns.RR{test.A("example.net.	86400	IN	A	93.184.216.34")},
		Extra: []dns.RR{test.OPT(4096, false)}, // This will have an EDNS0 section, because *we* added our local stub forward to detect loops.
	},
}
