package erratic

import (
	"strings"
	"sync"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// allRecords returns a small zone file. The first RR must be a SOA.
func allRecords(name string) []dns.RR {
	var rrs = []dns.RR{
		test.SOA("xx.		0	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2018050825 7200 3600 1209600 3600"),
		test.NS("xx.		0	IN	NS	b.xx."),
		test.NS("xx.		0	IN	NS	a.xx."),
		test.AAAA("a.xx.	0	IN	AAAA	2001:bd8::53"),
		test.AAAA("b.xx.	0	IN	AAAA	2001:500::54"),
	}

	for _, r := range rrs {
		r.Header().Name = strings.Replace(r.Header().Name, "xx.", name, 1)

		if n, ok := r.(*dns.NS); ok {
			n.Ns = strings.Replace(n.Ns, "xx.", name, 1)
		}
	}
	return rrs
}

func xfr(state request.Request, truncate bool) {
	rrs := allRecords(state.QName())

	ch := make(chan *dns.Envelope)
	tr := new(dns.Transfer)

	go func() {
		// So the rrs we have don't have a closing SOA, only add that when truncate is false,
		// so we send an incomplete AXFR.
		if !truncate {
			rrs = append(rrs, rrs[0])
		}

		ch <- &dns.Envelope{RR: rrs}
		close(ch)
	}()

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		tr.Out(state.W, state.Req, ch)
		wg.Done()
	}()
	wg.Wait()
}
