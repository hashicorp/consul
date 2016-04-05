package metrics

import (
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

func (m *Metrics) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}

	qname := state.Name()
	qtype := state.Type()
	zone := middleware.Zones(m.ZoneNames).Matches(qname)
	if zone == "" {
		zone = "."
	}

	// Record response to get status code and size of the reply.
	rw := middleware.NewResponseRecorder(w)
	status, err := m.Next.ServeDNS(ctx, rw, r)

	requestCount.WithLabelValues(zone, qtype).Inc()
	requestDuration.WithLabelValues(zone, qtype).Observe(float64(time.Since(rw.Start()) / time.Second))
	responseSize.WithLabelValues(zone, qtype).Observe(float64(rw.Size()))
	responseRcode.WithLabelValues(zone, strconv.Itoa(rw.Rcode()), qtype).Inc()

	return status, err
}
