// Package clouddns implements a plugin that returns resource records
// from GCP Cloud DNS.
package clouddns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	gcp "google.golang.org/api/dns/v1"
)

// CloudDNS is a plugin that returns RR from GCP Cloud DNS.
type CloudDNS struct {
	Next plugin.Handler
	Fall fall.F

	zoneNames []string
	client    gcpDNS
	upstream  *upstream.Upstream

	zMu   sync.RWMutex
	zones zones
}

type zone struct {
	projectName string
	zoneName    string
	z           *file.Zone
	dns         string
}

type zones map[string][]*zone

// New reads from the keys map which uses domain names as its key and a colon separated
// string of project name and hosted zone name lists as its values, validates
// that each domain name/zone id pair does exist, and returns a new *CloudDNS.
// In addition to this, upstream is passed for doing recursive queries against CNAMEs.
// Returns error if it cannot verify any given domain name/zone id pair.
func New(ctx context.Context, c gcpDNS, keys map[string][]string, up *upstream.Upstream) (*CloudDNS, error) {
	zones := make(map[string][]*zone, len(keys))
	zoneNames := make([]string, 0, len(keys))
	for dnsName, hostedZoneDetails := range keys {
		for _, hostedZone := range hostedZoneDetails {
			ss := strings.SplitN(hostedZone, ":", 2)
			if len(ss) != 2 {
				return nil, errors.New("either project or zone name missing")
			}
			err := c.zoneExists(ss[0], ss[1])
			if err != nil {
				return nil, err
			}
			fqdnDNSName := dns.Fqdn(dnsName)
			if _, ok := zones[fqdnDNSName]; !ok {
				zoneNames = append(zoneNames, fqdnDNSName)
			}
			zones[fqdnDNSName] = append(zones[fqdnDNSName], &zone{projectName: ss[0], zoneName: ss[1], dns: fqdnDNSName, z: file.NewZone(fqdnDNSName, "")})
		}
	}
	return &CloudDNS{
		client:    c,
		zoneNames: zoneNames,
		zones:     zones,
		upstream:  up,
	}, nil
}

// Run executes first update, spins up an update forever-loop.
// Returns error if first update fails.
func (h *CloudDNS) Run(ctx context.Context) error {
	if err := h.updateZones(ctx); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Infof("Breaking out of CloudDNS update loop: %v", ctx.Err())
				return
			case <-time.After(1 * time.Minute):
				if err := h.updateZones(ctx); err != nil && ctx.Err() == nil /* Don't log error if ctx expired. */ {
					log.Errorf("Failed to update zones: %v", err)
				}
			}
		}
	}()
	return nil
}

// ServeDNS implements the plugin.Handler interface.
func (h *CloudDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	zName := plugin.Zones(h.zoneNames).Matches(qname)
	if zName == "" {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	z, ok := h.zones[zName] // ok true if we are authoritative for the zone
	if !ok || z == nil {
		return dns.RcodeServerFailure, nil
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	var result file.Result

	for _, hostedZone := range z {
		h.zMu.RLock()
		m.Answer, m.Ns, m.Extra, result = hostedZone.z.Lookup(ctx, state, qname)
		h.zMu.RUnlock()

		// Take the answer if it's non-empty OR if there is another
		// record type exists for this name (NODATA).
		if len(m.Answer) != 0 || result == file.NoData {
			break
		}
	}

	if len(m.Answer) == 0 && result != file.NoData && h.Fall.Through(qname) {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	switch result {
	case file.Success:
	case file.NoData:
	case file.NameError:
		m.Rcode = dns.RcodeNameError
	case file.Delegation:
		m.Authoritative = false
	case file.ServerFailure:
		return dns.RcodeServerFailure, nil
	}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

func updateZoneFromRRS(rrs *gcp.ResourceRecordSetsListResponse, z *file.Zone) error {
	for _, rr := range rrs.Rrsets {
		var rfc1035 string
		var r dns.RR
		var err error
		for _, value := range rr.Rrdatas {
			if rr.Type == "CNAME" || rr.Type == "PTR" {
				value = dns.Fqdn(value)
			}

			// Assemble RFC 1035 conforming record to pass into dns scanner.
			rfc1035 = fmt.Sprintf("%s %d IN %s %s", dns.Fqdn(rr.Name), rr.Ttl, rr.Type, value)
			r, err = dns.NewRR(rfc1035)
			if err != nil {
				return fmt.Errorf("failed to parse resource record: %v", err)
			}
		}

		z.Insert(r)
	}
	return nil
}

// updateZones re-queries resource record sets for each zone and updates the
// zone object.
// Returns error if any zones error'ed out, but waits for other zones to
// complete first.
func (h *CloudDNS) updateZones(ctx context.Context) error {
	errc := make(chan error)
	defer close(errc)
	for zName, z := range h.zones {
		go func(zName string, z []*zone) {
			var err error
			var rrListResponse *gcp.ResourceRecordSetsListResponse
			defer func() {
				errc <- err
			}()

			for i, hostedZone := range z {
				newZ := file.NewZone(zName, "")
				newZ.Upstream = h.upstream
				rrListResponse, err = h.client.listRRSets(hostedZone.projectName, hostedZone.zoneName)
				if err != nil {
					err = fmt.Errorf("failed to list resource records for %v:%v:%v from gcp: %v", zName, hostedZone.projectName, hostedZone.zoneName, err)
					return
				}
				updateZoneFromRRS(rrListResponse, newZ)

				h.zMu.Lock()
				(*z[i]).z = newZ
				h.zMu.Unlock()
			}

		}(zName, z)
	}
	// Collect errors (if any). This will also sync on all zones updates
	// completion.
	var errs []string
	for i := 0; i < len(h.zones); i++ {
		err := <-errc
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("errors updating zones: %v", errs)
	}
	return nil
}

// Name implements the Handler interface.
func (h *CloudDNS) Name() string { return "clouddns" }
