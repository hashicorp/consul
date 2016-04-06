package metrics

import (
	"time"

	"golang.org/x/net/context"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

func (m Metrics) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
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

	Report(zone, qtype, rw.Rcode(), rw.Size(), rw.Start())

	return status, err
}

// Report is a plain reporting function that the server can use for REFUSED and other
// queries that are turned down because they don't match any middleware.
func Report(zone, qtype, rcode string, size int, start time.Time) {
	if requestCount == nil {
		// no metrics are enabled
		return
	}

	requestCount.WithLabelValues(zone, qtype).Inc()
	requestDuration.WithLabelValues(zone, qtype).Observe(float64(time.Since(start) / time.Second))
	responseSize.WithLabelValues(zone, qtype).Observe(float64(size))
	responseRcode.WithLabelValues(zone, rcode, qtype).Inc()
}
