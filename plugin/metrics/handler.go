package metrics

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics/vars"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/rcode"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// ServeDNS implements the Handler interface.
func (m *Metrics) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	qname := state.QName()
	zone := plugin.Zones(m.ZoneNames()).Matches(qname)
	if zone == "" {
		zone = "."
	}

	// Record response to get status code and size of the reply.
	rw := dnstest.NewRecorder(w)
	status, err := plugin.NextOrFailure(m.Name(), m.Next, ctx, rw, r)

	vars.Report(WithServer(ctx), state, zone, rcode.ToString(rw.Rcode), rw.Len, rw.Start)

	return status, err
}

// Name implements the Handler interface.
func (m *Metrics) Name() string { return "prometheus" }
