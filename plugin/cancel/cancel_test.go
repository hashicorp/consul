package cancel

import (
	"context"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

type sleepPlugin struct{}

func (s sleepPlugin) Name() string { return "sleep" }

func (s sleepPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	i := 0
	m := new(dns.Msg)
	m.SetReply(r)
	for {
		if plugin.Done(ctx) {
			m.Rcode = dns.RcodeBadTime // use BadTime to return something time related
			w.WriteMsg(m)
			return 0, nil
		}
		time.Sleep(20 * time.Millisecond)
		i++
		if i > 2 {
			m.Rcode = dns.RcodeServerFailure
			w.WriteMsg(m)
			return 0, nil
		}
	}
}

func TestCancel(t *testing.T) {
	ca := Cancel{Next: sleepPlugin{}, timeout: 20 * time.Millisecond}
	ctx := context.Background()

	w := dnstest.NewRecorder(&test.ResponseWriter{})
	m := new(dns.Msg)
	m.SetQuestion("aaa.example.com.", dns.TypeTXT)

	ca.ServeDNS(ctx, w, m)
	if w.Rcode != dns.RcodeBadTime {
		t.Error("Expected ServeDNS to be canceled by context")
	}
}
