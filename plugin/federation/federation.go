/*
Package federation implements kubernetes federation. It checks if the qname matches
a possible federation. If this is the case and the captured answer is an NXDOMAIN,
federation is performed. If this is not the case the original answer is returned.

The federation label is always the 2nd to last once the zone is chopped of. For
instance "nginx.mynamespace.myfederation.svc.example.com" has "myfederation" as
the federation label. For federation to work we do a normal k8s lookup
*without* that label, if that comes back with NXDOMAIN or NODATA(??) we create
a federation record and return that.

Federation is only useful in conjunction with the kubernetes plugin, without it is a noop.
*/
package federation

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Federation contains the name to zone mapping used for federation in kubernetes.
type Federation struct {
	f     map[string]string
	zones []string

	Next        plugin.Handler
	Federations Func
}

// Func needs to be implemented by any plugin that implements
// federation. Right now this is only the kubernetes plugin.
type Func func(state request.Request, fname, fzone string) (msg.Service, error)

// New returns a new federation.
func New() *Federation {
	return &Federation{f: make(map[string]string)}
}

// ServeDNS implements the plugin.Handle interface.
func (f *Federation) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	if f.Federations == nil {
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, w, r)
	}

	state := request.Request{W: w, Req: r}
	zone := plugin.Zones(f.zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, w, r)
	}

	state.Zone = zone

	// Remove the federation label from the qname to see if something exists.
	without, label := f.isNameFederation(state.Name(), state.Zone)
	if without == "" {
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, w, r)
	}

	qname := r.Question[0].Name
	r.Question[0].Name = without
	state.Clear()

	// Start the next plugin, but with a nowriter, capture the result, if NXDOMAIN
	// perform federation, otherwise just write the result.
	nw := nonwriter.New(w)
	ret, err := plugin.NextOrFailure(f.Name(), f.Next, ctx, nw, r)

	if !plugin.ClientWrite(ret) {
		// something went wrong
		r.Question[0].Name = qname
		return ret, err
	}

	if m := nw.Msg; m.Rcode != dns.RcodeNameError {
		// If positive answer we need to substitute the original qname in the answer.
		m.Question[0].Name = qname
		for _, a := range m.Answer {
			a.Header().Name = qname
		}

		state.SizeAndDo(m)
		m, _ = state.Scrub(m)
		w.WriteMsg(m)

		return dns.RcodeSuccess, nil
	}

	// Still here, we've seen NXDOMAIN and need to perform federation.
	service, err := f.Federations(state, label, f.f[label]) // state references Req which has updated qname
	if err != nil {
		r.Question[0].Name = qname
		return dns.RcodeServerFailure, err
	}

	r.Question[0].Name = qname

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	m.Answer = []dns.RR{service.NewCNAME(state.QName(), service.Host)}

	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)

	return dns.RcodeSuccess, nil
}

// Name implements the plugin.Handle interface.
func (f *Federation) Name() string { return "federation" }

// IsNameFederation checks the qname to see if it is a potential federation. The federation
// label is always the 2nd to last once the zone is chopped of. For instance
// "nginx.mynamespace.myfederation.svc.example.com" has "myfederation" as the federation label.
// IsNameFederation returns a new qname with the federation label and the label itself or two
// empty strings if there wasn't a hit.
func (f *Federation) isNameFederation(name, zone string) (string, string) {
	base, _ := dnsutil.TrimZone(name, zone)

	// TODO(miek): dns.PrevLabel is better for memory, or dns.Split.
	labels := dns.SplitDomainName(base)
	ll := len(labels)
	if ll < 2 {
		return "", ""
	}

	fed := labels[ll-2]

	if _, ok := f.f[fed]; ok {
		without := dnsutil.Join(labels[:ll-2]) + labels[ll-1] + "." + zone
		return without, fed
	}
	return "", ""
}
