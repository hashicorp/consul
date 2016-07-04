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

	typ := state.QType()

	requestCount.WithLabelValues(zone, net, fam).Inc()
	requestDuration.WithLabelValues(zone).Observe(float64(time.Since(start) / time.Millisecond))

	if state.Do() {
		requestDo.WithLabelValues(zone).Inc()
	}

	if _, known := monitorType[typ]; known {
		requestType.WithLabelValues(zone, dns.Type(typ).String()).Inc()
	} else {
		requestType.WithLabelValues(zone, other).Inc()
	}

	if typ == dns.TypeIXFR || typ == dns.TypeAXFR {
		responseTransferSize.WithLabelValues(zone, net).Observe(float64(size))
		requestTransferSize.WithLabelValues(zone, net).Observe(float64(state.Size()))
	} else {
		responseSize.WithLabelValues(zone, net).Observe(float64(size))
		requestSize.WithLabelValues(zone, net).Observe(float64(state.Size()))
	}

	responseRcode.WithLabelValues(zone, rcode).Inc()
}

var monitorType = map[uint16]bool{
	dns.TypeAAAA:   true,
	dns.TypeA:      true,
	dns.TypeCNAME:  true,
	dns.TypeDNSKEY: true,
	dns.TypeDS:     true,
	dns.TypeMX:     true,
	dns.TypeNSEC3:  true,
	dns.TypeNSEC:   true,
	dns.TypeNS:     true,
	dns.TypePTR:    true,
	dns.TypeRRSIG:  true,
	dns.TypeSOA:    true,
	dns.TypeSRV:    true,
	dns.TypeTXT:    true,
	// Meta Qtypes
	dns.TypeIXFR: true,
	dns.TypeAXFR: true,
	dns.TypeANY:  true,
}

const other = "other"
