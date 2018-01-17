package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"

	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestPrefetch(t *testing.T) {
	tests := []struct {
		qname         string
		ttl           int
		prefetch      int
		verifications []verification
	}{
		{
			qname:    "hits.reset.example.org.",
			ttl:      80,
			prefetch: 1,
			verifications: []verification{
				{
					after:  0 * time.Second,
					answer: "hits.reset.example.org. 80 IN A 127.0.0.1",
					fetch:  true,
				},
				{
					after:  73 * time.Second,
					answer: "hits.reset.example.org.  7 IN A 127.0.0.1",
					fetch:  true,
				},
				{
					after:  80 * time.Second,
					answer: "hits.reset.example.org. 73 IN A 127.0.0.2",
				},
			},
		},
		{
			qname:    "short.ttl.example.org.",
			ttl:      5,
			prefetch: 1,
			verifications: []verification{
				{
					after:  0 * time.Second,
					answer: "short.ttl.example.org. 5 IN A 127.0.0.1",
					fetch:  true,
				},
				{
					after:  1 * time.Second,
					answer: "short.ttl.example.org. 4 IN A 127.0.0.1",
				},
				{
					after:  4 * time.Second,
					answer: "short.ttl.example.org. 1 IN A 127.0.0.1",
					fetch:  true,
				},
				{
					after:  5 * time.Second,
					answer: "short.ttl.example.org. 4 IN A 127.0.0.2",
				},
			},
		},
		{
			qname:    "no.prefetch.example.org.",
			ttl:      30,
			prefetch: 0,
			verifications: []verification{
				{
					after:  0 * time.Second,
					answer: "no.prefetch.example.org. 30 IN A 127.0.0.1",
					fetch:  true,
				},
				{
					after:  15 * time.Second,
					answer: "no.prefetch.example.org. 15 IN A 127.0.0.1",
				},
				{
					after:  29 * time.Second,
					answer: "no.prefetch.example.org.  1 IN A 127.0.0.1",
				},
				{
					after:  30 * time.Second,
					answer: "no.prefetch.example.org. 30 IN A 127.0.0.2",
					fetch:  true,
				},
			},
		},
	}

	t0, err := time.Parse(time.RFC3339, "2018-01-01T14:00:00+00:00")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.qname, func(t *testing.T) {
			fetchc := make(chan struct{}, 1)

			c := New()
			c.prefetch = tt.prefetch
			c.Next = prefetchHandler(tt.qname, tt.ttl, fetchc)

			req := new(dns.Msg)
			req.SetQuestion(tt.qname, dns.TypeA)
			rec := dnstest.NewRecorder(&test.ResponseWriter{})

			for _, v := range tt.verifications {
				c.now = func() time.Time { return t0.Add(v.after) }

				c.ServeDNS(context.TODO(), rec, req)
				if v.fetch {
					select {
					case <-fetchc:
						if !v.fetch {
							t.Fatalf("after %s: want request to trigger a prefetch", v.after)
						}
					case <-time.After(time.Second):
						t.Fatalf("after %s: want request to trigger a prefetch", v.after)
					}
				}
				if want, got := rec.Rcode, dns.RcodeSuccess; want != got {
					t.Errorf("after %s: want rcode %d, got %d", v.after, want, got)
				}
				if want, got := 1, len(rec.Msg.Answer); want != got {
					t.Errorf("after %s: want %d answer RR, got %d", v.after, want, got)
				}
				if want, got := test.A(v.answer).String(), rec.Msg.Answer[0].String(); want != got {
					t.Errorf("after %s: want answer %s, got %s", v.after, want, got)
				}
			}
		})
	}
}

type verification struct {
	after  time.Duration
	answer string
	// fetch defines whether a request is sent to the next handler.
	fetch bool
}

// prefetchHandler is a fake plugin implementation which returns a single A
// record with the given qname and ttl. The returned IP address starts at
// 127.0.0.1 and is incremented on every request.
func prefetchHandler(qname string, ttl int, fetchc chan struct{}) plugin.Handler {
	i := 0
	return plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
		i++
		m := new(dns.Msg)
		m.SetQuestion(qname, dns.TypeA)
		m.Response = true
		m.Answer = append(m.Answer, test.A(fmt.Sprintf("%s %d IN A 127.0.0.%d", qname, ttl, i)))

		w.WriteMsg(m)
		fetchc <- struct{}{}
		return dns.RcodeSuccess, nil
	})
}
