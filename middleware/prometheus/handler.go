package metrics

import (
	"strconv"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

func (m *Metrics) ServeDNS(w dns.ResponseWriter, r *dns.Msg) (int, error) {
	context := middleware.Context{W: w, Req: r}

	qname := context.Name()
	qtype := context.Type()
	zone := middleware.Zones(m.ZoneNames).Matches(qname)
	if zone == "" {
		zone = "."
	}

	// Record response to get status code and size of the reply.
	rw := middleware.NewResponseRecorder(w)
	status, err := m.Next.ServeDNS(rw, r)

	requestCount.WithLabelValues(zone, qtype).Inc()
	requestDuration.WithLabelValues(zone).Observe(float64(time.Since(rw.Start()) / time.Second))
	responseSize.WithLabelValues(zone).Observe(float64(rw.Size()))
	responseRcode.WithLabelValues(zone, strconv.Itoa(rw.Rcode())).Inc()

	return status, err
}
