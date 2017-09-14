package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/cache"
	"github.com/coredns/coredns/plugin/pkg/dnsrecorder"

	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var p = false

func TestPrefetch(t *testing.T) {
	c := &Cache{Zones: []string{"."}, pcap: defaultCap, ncap: defaultCap, pttl: maxTTL, nttl: maxTTL}
	c.pcache = cache.New(c.pcap)
	c.ncache = cache.New(c.ncap)
	c.prefetch = 1
	c.duration = 1 * time.Second
	c.Next = PrefetchHandler(t, dns.RcodeSuccess, nil)

	ctx := context.TODO()

	req := new(dns.Msg)
	req.SetQuestion("lowttl.example.org.", dns.TypeA)

	rec := dnsrecorder.New(&test.ResponseWriter{})

	c.ServeDNS(ctx, rec, req)
	p = true // prefetch should be true for the 2nd fetch
	c.ServeDNS(ctx, rec, req)
}

func PrefetchHandler(t *testing.T, rcode int, err error) plugin.Handler {
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		m := new(dns.Msg)
		m.SetQuestion("lowttl.example.org.", dns.TypeA)
		m.Response = true
		m.RecursionAvailable = true
		m.Answer = append(m.Answer, test.A("lowttl.example.org. 80 IN A 127.0.0.53"))
		if p != w.(*ResponseWriter).prefetch {
			err = fmt.Errorf("cache prefetch not equal to p: got %t, want %t", p, w.(*ResponseWriter).prefetch)
			t.Fatal(err)
		}

		w.WriteMsg(m)
		return rcode, err
	})
}
