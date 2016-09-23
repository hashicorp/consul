package file

import (
	"fmt"
	"log"

	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Xfr serves up an AXFR.
type Xfr struct {
	*Zone
}

// ServeDNS implements the middleware.Handler interface.
func (x Xfr) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	if !x.TransferAllowed(state) {
		return dns.RcodeServerFailure, nil
	}
	if state.QType() != dns.TypeAXFR && state.QType() != dns.TypeIXFR {
		return 0, fmt.Errorf("xfr called with non transfer type: %d", state.QType())
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
	records = append(records, records[0]) // add closing SOA to the end
	log.Printf("[INFO] Outgoing transfer of %d records of zone %s to %s started", len(records), x.origin, state.IP())
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

const transferLength = 1000 // Start a new envelop after message reaches this size in bytes. Intentionally small to test multi envelope parsing.
