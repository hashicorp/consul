// Package file implements a file backend.
package file

import (
	"fmt"
	"io"
	"log"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type (
	// File is the plugin that reads zone data from disk.
	File struct {
		Next  plugin.Handler
		Zones Zones
	}

	// Zones maps zone names to a *Zone.
	Zones struct {
		Z     map[string]*Zone // A map mapping zone (origin) to the Zone's data
		Names []string         // All the keys from the map Z as a string slice.
	}
)

// ServeDNS implements the plugin.Handle interface.
func (f File) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	qname := state.Name()
	// TODO(miek): match the qname better in the map
	zone := plugin.Zones(f.Zones.Names).Matches(qname)
	if zone == "" {
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, w, r)
	}

	z, ok := f.Zones.Z[zone]
	if !ok || z == nil {
		return dns.RcodeServerFailure, nil
	}

	// This is only for when we are a secondary zones.
	if r.Opcode == dns.OpcodeNotify {
		if z.isNotify(state) {
			m := new(dns.Msg)
			m.SetReply(r)
			m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
			state.SizeAndDo(m)
			w.WriteMsg(m)

			log.Printf("[INFO] Notify from %s for %s: checking transfer", state.IP(), zone)
			ok, err := z.shouldTransfer()
			if ok {
				z.TransferIn()
			} else {
				log.Printf("[INFO] Notify from %s for %s: no serial increase seen", state.IP(), zone)
			}
			if err != nil {
				log.Printf("[WARNING] Notify from %s for %s: failed primary check: %s", state.IP(), zone, err)
			}
			return dns.RcodeSuccess, nil
		}
		log.Printf("[INFO] Dropping notify from %s for %s", state.IP(), zone)
		return dns.RcodeSuccess, nil
	}

	if z.Expired != nil && *z.Expired {
		log.Printf("[ERROR] Zone %s is expired", zone)
		return dns.RcodeServerFailure, nil
	}

	if state.QType() == dns.TypeAXFR || state.QType() == dns.TypeIXFR {
		xfr := Xfr{z}
		return xfr.ServeDNS(ctx, w, r)
	}

	answer, ns, extra, result := z.Lookup(state, qname)

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Answer, m.Ns, m.Extra = answer, ns, extra

	switch result {
	case Success:
	case NoData:
	case NameError:
		m.Rcode = dns.RcodeNameError
	case Delegation:
		m.Authoritative = false
	case ServerFailure:
		return dns.RcodeServerFailure, nil
	}

	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (f File) Name() string { return "file" }

type serialErr struct {
	err    string
	zone   string
	origin string
	serial int64
}

func (s *serialErr) Error() string {
	return fmt.Sprintf("%s for origin %s in file %s, with serial %d", s.err, s.origin, s.zone, s.serial)
}

// Parse parses the zone in filename and returns a new Zone or an error.
// If serial >= 0 it will reload the zone, if the SOA hasn't changed
// it returns an error indicating nothing was read.
func Parse(f io.Reader, origin, fileName string, serial int64) (*Zone, error) {
	tokens := dns.ParseZone(f, dns.Fqdn(origin), fileName)
	z := NewZone(origin, fileName)
	seenSOA := false
	for x := range tokens {
		if x.Error != nil {
			return nil, x.Error
		}

		if !seenSOA && serial >= 0 {
			if s, ok := x.RR.(*dns.SOA); ok {
				if s.Serial == uint32(serial) { // same serial
					return nil, &serialErr{err: "no change in SOA serial", origin: origin, zone: fileName, serial: serial}
				}
				seenSOA = true
			}
		}

		if err := z.Insert(x.RR); err != nil {
			return nil, err
		}
	}
	if !seenSOA {
		return nil, fmt.Errorf("file %q has no SOA record", fileName)
	}

	return z, nil
}
