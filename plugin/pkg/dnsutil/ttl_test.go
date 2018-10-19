package dnsutil

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/response"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// See https://github.com/kubernetes/dns/issues/121, add some specific tests for those use cases.

func TestMinimalTTL(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion("z.alm.im.", dns.TypeA)
	m.Ns = []dns.RR{
		test.SOA("alm.im.	1800	IN	SOA	ivan.ns.cloudflare.com. dns.cloudflare.com. 2025042470 10000 2400 604800 3600"),
	}

	utc := time.Now().UTC()

	mt, _ := response.Typify(m, utc)
	if mt != response.NoData {
		t.Fatalf("Expected type to be response.NoData, got %s", mt)
	}
	dur := MinimalTTL(m, mt) // minTTL on msg is 3600 (neg. ttl on SOA)
	if dur != time.Duration(1800*time.Second) {
		t.Fatalf("Expected minttl duration to be %d, got %d", 1800, dur)
	}

	m.Rcode = dns.RcodeNameError
	mt, _ = response.Typify(m, utc)
	if mt != response.NameError {
		t.Fatalf("Expected type to be response.NameError, got %s", mt)
	}
	dur = MinimalTTL(m, mt) // minTTL on msg is 3600 (neg. ttl on SOA)
	if dur != time.Duration(1800*time.Second) {
		t.Fatalf("Expected minttl duration to be %d, got %d", 1800, dur)
	}
}

func BenchmarkMinimalTTL(b *testing.B) {
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	m.Ns = []dns.RR{
		test.A("a.example.org. 	1800	IN	A 127.0.0.53"),
		test.A("b.example.org. 	1900	IN	A 127.0.0.53"),
		test.A("c.example.org. 	1600	IN	A 127.0.0.53"),
		test.A("d.example.org. 	1100	IN	A 127.0.0.53"),
		test.A("e.example.org. 	1000	IN	A 127.0.0.53"),
	}
	m.Extra = []dns.RR{
		test.A("a.example.org. 	1800	IN	A 127.0.0.53"),
		test.A("b.example.org. 	1600	IN	A 127.0.0.53"),
		test.A("c.example.org. 	1400	IN	A 127.0.0.53"),
		test.A("d.example.org. 	1200	IN	A 127.0.0.53"),
		test.A("e.example.org. 	1100	IN	A 127.0.0.53"),
	}

	utc := time.Now().UTC()
	mt, _ := response.Typify(m, utc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dur := MinimalTTL(m, mt)
		if dur != 1000*time.Second {
			b.Fatalf("Wrong MinimalTTL %d, expected %d", dur, 1000*time.Second)
		}
	}
}
