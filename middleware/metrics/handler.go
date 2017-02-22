package metrics

import (
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/metrics/vars"
	"github.com/coredns/coredns/middleware/pkg/dnsrecorder"
	"github.com/coredns/coredns/middleware/pkg/rcode"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the Handler interface.
func (m *Metrics) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	qname := state.QName()
	zone := middleware.Zones(m.ZoneNames()).Matches(qname)
	if zone == "" {
		zone = "."
	}

	// Record response to get status code and size of the reply.
	rw := dnsrecorder.New(w)
	status, err := middleware.NextOrFailure(m.Name(), m.Next, ctx, rw, r)

	vars.Report(state, zone, rcode.ToString(rw.Rcode), rw.Len, rw.Start)

	return status, err
}

// Name implements the Handler interface.
func (m *Metrics) Name() string { return "prometheus" }
