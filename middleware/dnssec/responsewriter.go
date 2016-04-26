package dnssec

import (
	"log"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
)

type DnssecResponseWriter struct {
	dns.ResponseWriter
	d Dnssec
}

func NewDnssecResponseWriter(w dns.ResponseWriter, d Dnssec) *DnssecResponseWriter {
	return &DnssecResponseWriter{w, d}
}

func (d *DnssecResponseWriter) WriteMsg(res *dns.Msg) error {
	// By definition we should sign anything that comes back, we should still figure out for
	// which zone it should be.
	state := middleware.State{W: d.ResponseWriter, Req: res}

	qname := state.Name()
	zone := middleware.Zones(d.d.zones).Matches(qname)
	if zone == "" {
		return d.ResponseWriter.WriteMsg(res)
	}

	if state.Do() {
		res = d.d.Sign(state, zone, time.Now().UTC())
	}
	state.SizeAndDo(res)

	return d.ResponseWriter.WriteMsg(res)
}

func (d *DnssecResponseWriter) Write(buf []byte) (int, error) {
	log.Printf("[WARNING] Dnssec called with Write: not signing reply")
	n, err := d.ResponseWriter.Write(buf)
	return n, err
}

func (d *DnssecResponseWriter) Hijack() {
	d.ResponseWriter.Hijack()
	return
}
