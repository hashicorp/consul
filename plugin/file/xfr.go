package file

import (
	"context"
	"fmt"
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Xfr serves up an AXFR.
type Xfr struct {
	*Zone
}

// ServeDNS implements the plugin.Handler interface.
func (x Xfr) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	if !x.TransferAllowed(state) {
		return dns.RcodeServerFailure, nil
	}
	if state.QType() != dns.TypeAXFR && state.QType() != dns.TypeIXFR {
		return 0, plugin.Error(x.Name(), fmt.Errorf("xfr called with non transfer type: %d", state.QType()))
	}

	// For IXFR we take the SOA in the IXFR message (if there), compare it what we have and then decide to do an
	// AXFR or just reply with one SOA message back.
	if state.QType() == dns.TypeIXFR {
		code, _ := x.ServeIxfr(ctx, w, r)
		if plugin.ClientWrite(code) {
			return code, nil
		}
	}

	records := x.All()
	if len(records) == 0 {
		return dns.RcodeServerFailure, nil
	}

	ch := make(chan *dns.Envelope)
	tr := new(dns.Transfer)
	wg := new(sync.WaitGroup)
	go func() {
		wg.Add(1)
		tr.Out(w, r, ch)
		wg.Done()
	}()

	j, l := 0, 0
	records = append(records, records[0]) // add closing SOA to the end
	log.Infof("Outgoing transfer of %d records of zone %s to %s started with %d SOA serial", len(records), x.origin, state.IP(), x.SOASerialIfDefined())
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
	close(ch) // Even though we close the channel here, we still have
	wg.Wait() // to wait before we can return and close the connection.

	return dns.RcodeSuccess, nil
}

// Name implements the plugin.Handler interface.
func (x Xfr) Name() string { return "xfr" }

// ServeIxfr checks if we need to serve a simpler IXFR for the incoming message.
// See RFC 1995 Section 3: "... and the authority section containing the SOA record of client's version of the zone."
// and Section 2, paragraph 4 where we only need to echo the SOA record back.
// This function must be called when the qtype is IXFR. It returns a plugin.ClientWrite(code) == false, when it didn't
// write anything and we should perform an AXFR.
func (x Xfr) ServeIxfr(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	if len(r.Ns) != 1 {
		return dns.RcodeServerFailure, nil
	}
	soa, ok := r.Ns[0].(*dns.SOA)
	if !ok {
		return dns.RcodeServerFailure, nil
	}

	x.RLock()
	if x.Apex.SOA == nil {
		x.RUnlock()
		return dns.RcodeServerFailure, nil
	}
	serial := x.Apex.SOA.Serial
	x.RUnlock()

	if soa.Serial == serial { // Section 2, para 4; echo SOA back. We have the same zone
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = []dns.RR{soa}
		w.WriteMsg(m)
		return 0, nil
	}
	return dns.RcodeServerFailure, nil
}

const transferLength = 1000 // Start a new envelop after message reaches this size in bytes. Intentionally small to test multi envelope parsing.
