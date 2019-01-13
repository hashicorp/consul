// Package route53 implements a plugin that returns resource records
// from AWS route53.
package route53

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/miekg/dns"
)

// Route53 is a plugin that returns RR from AWS route53.
type Route53 struct {
	Next plugin.Handler
	Fall fall.F

	zoneNames []string
	client    route53iface.Route53API
	upstream  *upstream.Upstream

	zMu   sync.RWMutex
	zones zones
}

type zone struct {
	id  string
	z   *file.Zone
	dns string
}

type zones map[string][]*zone

// New reads from the keys map which uses domain names as its key and hosted
// zone id lists as its values, validates that each domain name/zone id pair does
// exist, and returns a new *Route53. In addition to this, upstream is passed
// for doing recursive queries against CNAMEs.
// Returns error if it cannot verify any given domain name/zone id pair.
func New(ctx context.Context, c route53iface.Route53API, keys map[string][]string, up *upstream.Upstream) (*Route53, error) {
	zones := make(map[string][]*zone, len(keys))
	zoneNames := make([]string, 0, len(keys))
	for dns, hostedZoneIDs := range keys {
		for _, hostedZoneID := range hostedZoneIDs {
			_, err := c.ListHostedZonesByNameWithContext(ctx, &route53.ListHostedZonesByNameInput{
				DNSName:      aws.String(dns),
				HostedZoneId: aws.String(hostedZoneID),
			})
			if err != nil {
				return nil, err
			}
			if _, ok := zones[dns]; !ok {
				zoneNames = append(zoneNames, dns)
			}
			zones[dns] = append(zones[dns], &zone{id: hostedZoneID, dns: dns, z: file.NewZone(dns, "")})
		}
	}
	return &Route53{
		client:    c,
		zoneNames: zoneNames,
		zones:     zones,
		upstream:  up,
	}, nil
}

// Run executes first update, spins up an update forever-loop.
// Returns error if first update fails.
func (h *Route53) Run(ctx context.Context) error {
	if err := h.updateZones(ctx); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Infof("Breaking out of Route53 update loop: %v", ctx.Err())
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

// ServeDNS implements the plugin.Handler.ServeDNS.
func (h *Route53) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	zName := plugin.Zones(h.zoneNames).Matches(qname)
	if zName == "" {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}
	z, ok := h.zones[zName]
	if !ok || z == nil {
		return dns.RcodeServerFailure, nil
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	var result file.Result
	for _, hostedZone := range z {
		h.zMu.RLock()
		m.Answer, m.Ns, m.Extra, result = hostedZone.z.Lookup(state, qname)
		h.zMu.RUnlock()
		if len(m.Answer) != 0 {
			break
		}
	}

	if len(m.Answer) == 0 && h.Fall.Through(qname) {
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

func updateZoneFromRRS(rrs *route53.ResourceRecordSet, z *file.Zone) error {
	for _, rr := range rrs.ResourceRecords {
		// Assemble RFC 1035 conforming record to pass into dns scanner.
		rfc1035 := fmt.Sprintf("%s %d IN %s %s", aws.StringValue(rrs.Name), aws.Int64Value(rrs.TTL), aws.StringValue(rrs.Type), aws.StringValue(rr.Value))
		r, err := dns.NewRR(rfc1035)
		if err != nil {
			return fmt.Errorf("failed to parse resource record: %v", err)
		}

		z.Insert(r)
	}
	return nil
}

// updateZones re-queries resource record sets for each zone and updates the
// zone object.
// Returns error if any zones error'ed out, but waits for other zones to
// complete first.
func (h *Route53) updateZones(ctx context.Context) error {
	errc := make(chan error)
	defer close(errc)
	for zName, z := range h.zones {
		go func(zName string, z []*zone) {
			var err error
			defer func() {
				errc <- err
			}()

			for i, hostedZone := range z {
				newZ := file.NewZone(zName, "")
				newZ.Upstream = h.upstream
				in := &route53.ListResourceRecordSetsInput{
					HostedZoneId: aws.String(hostedZone.id),
				}
				err = h.client.ListResourceRecordSetsPagesWithContext(ctx, in,
					func(out *route53.ListResourceRecordSetsOutput, last bool) bool {
						for _, rrs := range out.ResourceRecordSets {
							if err := updateZoneFromRRS(rrs, newZ); err != nil {
								// Maybe unsupported record type. Log and carry on.
								log.Warningf("Failed to process resource record set: %v", err)
							}
						}
						return true
					})
				if err != nil {
					err = fmt.Errorf("failed to list resource records for %v:%v from route53: %v", zName, hostedZone.id, err)
					return
				}
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

// Name implements plugin.Handler.Name.
func (h *Route53) Name() string { return "route53" }
