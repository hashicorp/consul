package transfer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

var log = clog.NewWithPlugin("transfer")

// Transfer is a plugin that handles zone transfers.
type Transfer struct {
	Transferers []Transferer // the list of plugins that implement Transferer
	xfrs        []*xfr
	Next        plugin.Handler // the next plugin in the chain
}

type xfr struct {
	Zones []string
	to    []string
}

// Transferer may be implemented by plugins to enable zone transfers
type Transferer interface {
	// Transfer returns a channel to which it writes responses to the transfer request.
	// If the plugin is not authoritative for the zone, it should immediately return the
	// Transfer.ErrNotAuthoritative error.
	//
	// If serial is 0, handle as an AXFR request. Transfer should send all records
	// in the zone to the channel. The SOA should be written to the channel first, followed
	// by all other records, including all NS + glue records.
	//
	// If serial is not 0, handle as an IXFR request. If the serial is equal to or greater (newer) than
	// the current serial for the zone, send a single SOA record to the channel.
	// If the serial is less (older) than the current serial for the zone, perform an AXFR fallback
	// by proceeding as if an AXFR was requested (as above).
	Transfer(zone string, serial uint32) (<-chan []dns.RR, error)
}

var (
	// ErrNotAuthoritative is returned by Transfer() when the plugin is not authoritative for the zone
	ErrNotAuthoritative = errors.New("not authoritative for zone")
)

// ServeDNS implements the plugin.Handler interface.
func (t Transfer) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	if state.QType() != dns.TypeAXFR && state.QType() != dns.TypeIXFR {
		return plugin.NextOrFailure(t.Name(), t.Next, ctx, w, r)
	}

	// Find the first transfer instance for which the queried zone is a subdomain.
	var x *xfr
	for _, xfr := range t.xfrs {
		zone := plugin.Zones(xfr.Zones).Matches(state.Name())
		if zone == "" {
			continue
		}
		x = xfr
	}
	if x == nil {
		// Requested zone did not match any transfer instance zones.
		// Pass request down chain in case later plugins are capable of handling transfer requests themselves.
		return plugin.NextOrFailure(t.Name(), t.Next, ctx, w, r)
	}

	if !x.allowed(state) {
		return dns.RcodeRefused, nil
	}

	// Get serial from request if this is an IXFR
	var serial uint32
	if state.QType() == dns.TypeIXFR {
		soa, ok := r.Ns[0].(*dns.SOA)
		if !ok {
			return dns.RcodeServerFailure, nil
		}
		serial = soa.Serial
	}

	// Get a receiving channel from the first Transferer plugin that returns one
	var fromPlugin <-chan []dns.RR
	for _, p := range t.Transferers {
		var err error
		fromPlugin, err = p.Transfer(state.QName(), serial)
		if err == ErrNotAuthoritative {
			// plugin was not authoritative for the zone, try next plugin
			continue
		}
		if err != nil {
			return dns.RcodeServerFailure, err
		}
		break
	}

	if fromPlugin == nil {
		return plugin.NextOrFailure(t.Name(), t.Next, ctx, w, r)
	}

	// Send response to client
	ch := make(chan *dns.Envelope)
	tr := new(dns.Transfer)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		tr.Out(w, r, ch)
		wg.Done()
	}()

	var soa *dns.SOA
	rrs := []dns.RR{}
	l := 0

receive:
	for records := range fromPlugin {
		for _, record := range records {
			if soa == nil {
				if soa = record.(*dns.SOA); soa == nil {
					break receive
				}
				serial = soa.Serial
			}
			rrs = append(rrs, record)
			if len(rrs) > 500 {
				ch <- &dns.Envelope{RR: rrs}
				l += len(rrs)
				rrs = []dns.RR{}
			}
		}
	}

	if len(rrs) > 0 {
		ch <- &dns.Envelope{RR: rrs}
		l += len(rrs)
		rrs = []dns.RR{}
	}

	if soa != nil {
		ch <- &dns.Envelope{RR: []dns.RR{soa}} // closing SOA.
		l++
	}

	close(ch) // Even though we close the channel here, we still have
	wg.Wait() // to wait before we can return and close the connection.

	if soa == nil {
		return dns.RcodeServerFailure, fmt.Errorf("first record in zone %s is not SOA", state.QName())
	}

	log.Infof("Outgoing transfer of %d records of zone %s to %s with %d SOA serial", l, state.QName(), state.IP(), serial)
	return dns.RcodeSuccess, nil
}

func (x xfr) allowed(state request.Request) bool {
	for _, h := range x.to {
		if h == "*" {
			return true
		}
		to, _, err := net.SplitHostPort(h)
		if err != nil {
			return false
		}
		// If remote IP matches we accept.
		remote := state.IP()
		if to == remote {
			return true
		}
	}
	return false
}

// Name implements the Handler interface.
func (Transfer) Name() string { return "transfer" }
