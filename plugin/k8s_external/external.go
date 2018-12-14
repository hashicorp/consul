/*
Package external implements external names for kubernetes clusters.

This plugin only handles three qtypes (except the apex queries, because those are handled
differently). We support A, AAAA and SRV request, for all other types we return NODATA or
NXDOMAIN depending on the state of the cluster.

A plugin willing to provide these services must implement the Externaler interface, although it
likely only makes sense for the *kubernetes* plugin.

*/
package external

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Externaler defines the interface that a plugin should implement in order to be used by External.
type Externaler interface {
	// External returns a slice of msg.Services that are looked up in the backend and match
	// the request.
	External(request.Request) ([]msg.Service, int)
	// ExternalAddress should return a string slice of addresses for the nameserving endpoint.
	ExternalAddress(state request.Request) []dns.RR
}

// External resolves Ingress and Loadbalance IPs from kubernetes clusters.
type External struct {
	Next  plugin.Handler
	Zones []string

	hostmaster string
	apex       string
	ttl        uint32

	externalFunc     func(request.Request) ([]msg.Service, int)
	externalAddrFunc func(request.Request) []dns.RR
}

// New returns a new and initialized *External.
func New() *External {
	e := &External{hostmaster: "hostmaster", ttl: 5, apex: "dns"}
	return e
}

// ServeDNS implements the plugin.Handle interface.
func (e *External) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	zone := plugin.Zones(e.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	if e.externalFunc == nil {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	state.Zone = zone
	for _, z := range e.Zones {
		// TODO(miek): save this in the External struct.
		if state.Name() == z { // apex query
			ret, err := e.serveApex(state)
			return ret, err
		}
		if dns.IsSubDomain(e.apex+"."+z, state.Name()) {
			// dns subdomain test for ns. and dns. queries
			ret, err := e.serveSubApex(state)
			return ret, err
		}
	}

	svc, rcode := e.externalFunc(state)

	m := new(dns.Msg)
	m.SetReply(state.Req)

	if len(svc) == 0 {
		m.Rcode = rcode
		m.Ns = []dns.RR{e.soa(state)}
		w.WriteMsg(m)
		return 0, nil
	}

	switch state.QType() {
	case dns.TypeA:
		m.Answer = e.a(svc, state)
	case dns.TypeAAAA:
		m.Answer = e.aaaa(svc, state)
	case dns.TypeSRV:
		m.Answer, m.Extra = e.srv(svc, state)
	default:
		m.Ns = []dns.RR{e.soa(state)}
	}

	// If we did have records, but queried for the wrong qtype return a nodata response.
	if len(m.Answer) == 0 {
		m.Ns = []dns.RR{e.soa(state)}
	}

	w.WriteMsg(m)
	return 0, nil
}

// Name implements the Handler interface.
func (e *External) Name() string { return "k8s_external" }
