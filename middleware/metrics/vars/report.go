package vars

import (
	"time"

	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

// Report reports the metrics data associcated with request.
func Report(req request.Request, zone, rcode string, size int, start time.Time) {
	// Proto and Family
	net := req.Proto()
	fam := "1"
	if req.Family() == 2 {
		fam = "2"
	}

	typ := req.QType()

	RequestCount.WithLabelValues(zone, net, fam).Inc()
	RequestDuration.WithLabelValues(zone).Observe(float64(time.Since(start) / time.Millisecond))

	if req.Do() {
		RequestDo.WithLabelValues(zone).Inc()
	}

	if _, known := monitorType[typ]; known {
		RequestType.WithLabelValues(zone, dns.Type(typ).String()).Inc()
	} else {
		RequestType.WithLabelValues(zone, other).Inc()
	}

	ResponseSize.WithLabelValues(zone, net).Observe(float64(size))
	RequestSize.WithLabelValues(zone, net).Observe(float64(req.Size()))

	ResponseRcode.WithLabelValues(zone, rcode).Inc()
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
