package acl

import (
	"context"
	"net"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/request"

	"github.com/infobloxopen/go-trees/iptree"
	"github.com/miekg/dns"
)

// ACL enforces access control policies on DNS queries.
type ACL struct {
	Next plugin.Handler

	Rules []rule
}

// rule defines a list of Zones and some ACL policies which will be
// enforced on them.
type rule struct {
	zones    []string
	policies []policy
}

// action defines the action against queries.
type action int

// policy defines the ACL policy for DNS queries.
// A policy performs the specified action (block/allow) on all DNS queries
// matched by source IP or QTYPE.
type policy struct {
	action action
	qtypes map[uint16]struct{}
	filter *iptree.Tree
}

const (
	// actionNone does nothing on the queries.
	actionNone = iota
	// actionAllow allows authorized queries to recurse.
	actionAllow
	// actionBlock blocks unauthorized queries towards protected DNS zones.
	actionBlock
)

// ServeDNS implements the plugin.Handler interface.
func (a ACL) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

RulesCheckLoop:
	for _, rule := range a.Rules {
		// check zone.
		zone := plugin.Zones(rule.zones).Matches(state.Name())
		if zone == "" {
			continue
		}

		action := matchWithPolicies(rule.policies, w, r)
		switch action {
		case actionBlock:
			{
				m := new(dns.Msg)
				m.SetRcode(r, dns.RcodeRefused)
				w.WriteMsg(m)
				RequestBlockCount.WithLabelValues(metrics.WithServer(ctx), zone).Inc()
				return dns.RcodeSuccess, nil
			}
		case actionAllow:
			{
				break RulesCheckLoop
			}
		}
	}

	RequestAllowCount.WithLabelValues(metrics.WithServer(ctx)).Inc()
	return plugin.NextOrFailure(state.Name(), a.Next, ctx, w, r)
}

// matchWithPolicies matches the DNS query with a list of ACL polices and returns suitable
// action agains the query.
func matchWithPolicies(policies []policy, w dns.ResponseWriter, r *dns.Msg) action {
	state := request.Request{W: w, Req: r}

	ip := net.ParseIP(state.IP())
	qtype := state.QType()
	for _, policy := range policies {
		// dns.TypeNone matches all query types.
		_, matchAll := policy.qtypes[dns.TypeNone]
		_, match := policy.qtypes[qtype]
		if !matchAll && !match {
			continue
		}

		_, contained := policy.filter.GetByIP(ip)
		if !contained {
			continue
		}

		// matched.
		return policy.action
	}
	return actionNone
}

// Name implements the plugin.Handler interface.
func (a ACL) Name() string {
	return "acl"
}
