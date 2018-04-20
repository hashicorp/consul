package vars

import (
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"context"

	"github.com/miekg/dns"
)

// Report reports the metrics data associcated with request.
func Report(ctx context.Context, req request.Request, zone, rcode string, size int, start time.Time) {
	// Proto and Family.
	net := req.Proto()
	fam := "1"
	if req.Family() == 2 {
		fam = "2"
	}

	server := WithServer(ctx)

	typ := req.QType()
	RequestCount.WithLabelValues(server, zone, net, fam).Inc()
	RequestDuration.WithLabelValues(server, zone).Observe(time.Since(start).Seconds())

	if req.Do() {
		RequestDo.WithLabelValues(server, zone).Inc()
	}

	if _, known := monitorType[typ]; known {
		RequestType.WithLabelValues(server, zone, dns.Type(typ).String()).Inc()
	} else {
		RequestType.WithLabelValues(server, zone, other).Inc()
	}

	ResponseSize.WithLabelValues(server, zone, net).Observe(float64(size))
	RequestSize.WithLabelValues(server, zone, net).Observe(float64(req.Len()))

	ResponseRcode.WithLabelValues(server, zone, rcode).Inc()
}

// WithServer returns the current server handling the request.
func WithServer(ctx context.Context) string {
	srv := ctx.Value(plugin.ServerCtx{})
	if srv == nil {
		return ""
	}
	return srv.(string)
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
