package file

import (
	"fmt"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type (
	Xfr struct {
		*Zone
	}
)

// Serve an AXFR (or maybe later an IXFR) as well.
func (x Xfr) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}
	if !x.TransferAllowed(state) {
		return dns.RcodeServerFailure, nil
	}
	if state.QType() != dns.TypeAXFR {
		return 0, fmt.Errorf("file: xfr called with non xfr type: %d", state.QType())
	}
	if state.Proto() == "udp" {
		return 0, fmt.Errorf("file: xfr called with udp")
	}

	records := x.All()
	if len(records) == 0 {
		return dns.RcodeServerFailure, nil
	}

	ch := make(chan *dns.Envelope)
	defer close(ch)
	tr := new(dns.Transfer)
	go tr.Out(w, r, ch)

	j, l := 0, 0
	records = append(records, records[0])
	for i, r := range records {
		l += dns.Len(r)
		if l > transferLength {
			ch <- &dns.Envelope{RR: records[j:i]}
			l = 0
			j = i
		}
	}
	if j < len(records) {
		ch <- &dns.Envelope{RR: records[j:]}
	}

	w.Hijack()
	// w.Close() // Client closes connection
	return dns.RcodeSuccess, nil
}

//const transferLength = 10e3 // Start a new envelop after message reaches this size.
const transferLength = 100 // Start a new envelop after message reaches this size.
