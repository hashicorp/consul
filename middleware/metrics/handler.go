package metrics

import (
	"time"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func (m Metrics) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := middleware.State{W: w, Req: r}

	qname := state.QName()
	zone := middleware.Zones(m.ZoneNames).Matches(qname)
	if zone == "" {
		zone = "."
	}

	// Record response to get status code and size of the reply.
	rw := middleware.NewResponseRecorder(w)
	status, err := m.Next.ServeDNS(ctx, rw, r)

	Report(state, zone, rw.Rcode(), rw.Size(), rw.Start())

	return status, err
}

// Report is a plain reporting function that the server can use for REFUSED and other
// queries that are turned down because they don't match any middleware.
func Report(state middleware.State, zone, rcode string, size int, start time.Time) {
	if requestCount == nil {
		// no metrics are enabled
		return
	}

	// Proto and Family
	net := state.Proto()
	fam := "1"
	if state.Family() == 2 {
		fam = "2"
	}

	requestCount.WithLabelValues(zone, net, fam).Inc()
	requestDuration.WithLabelValues(zone).Observe(float64(time.Since(start) / time.Second))
	requestSize.WithLabelValues(zone).Observe(float64(state.Size()))
	if state.Do() {
		requestDo.WithLabelValues(zone).Inc()
	}

	responseSize.WithLabelValues(zone).Observe(float64(size))
	responseRcode.WithLabelValues(zone, rcode).Inc()
}
