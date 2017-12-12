/*
Package autopath implements autopathing. This is a hack; it shortcuts the
client's search path resolution by performing these lookups on the server...

The server has a copy (via AutoPathFunc) of the client's search path and on
receiving a query it first establish if the suffix matches the FIRST configured
element. If no match can be found the query will be forwarded up the plugin
chain without interference (iff 'fallthrough' has been set).

If the query is deemed to fall in the search path the server will perform the
queries with each element of the search path appended in sequence until a
non-NXDOMAIN answer has been found. That reply will then be returned to the
client - with some CNAME hackery to let the client accept the reply.

If all queries return NXDOMAIN we return the original as-is and let the client
continue searching. The client will go to the next element in the search path,
but we won’t do any more autopathing. It means that in the failure case, you do
more work, since the server looks it up, then the client still needs to go
through the search path.

It is assume the search path ordering is identical between server and client.

Midldeware implementing autopath, must have a function called `AutoPath` of type
autopath.Func. Note the searchpath must be ending with the empty string.

I.e:

func (m Plugins ) AutoPath(state request.Request) []string {
	return []string{"first", "second", "last", ""}
}
*/
package autopath

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Func defines the function plugin should implement to return a search
// path to the autopath plugin. The last element of the slice must be the empty string.
// If Func returns a nil slice, no autopathing will be done.
type Func func(request.Request) []string

// AutoPather defines the interface that a plugin should implement in order to be
// used by AutoPath.
type AutoPather interface {
	AutoPath(request.Request) []string
}

// AutoPath perform autopath: service side search path completion.
type AutoPath struct {
	Next  plugin.Handler
	Zones []string

	// Search always includes "" as the last element, so we try the base query with out any search paths added as well.
	search     []string
	searchFunc Func
}

// ServeDNS implements the plugin.Handle interface.
func (a *AutoPath) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	zone := plugin.Zones(a.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(a.Name(), a.Next, ctx, w, r)
	}

	// Check if autopath should be done, searchFunc takes precedence over the local configured search path.
	var err error
	searchpath := a.search

	if a.searchFunc != nil {
		searchpath = a.searchFunc(state)
	}

	if len(searchpath) == 0 {
		return plugin.NextOrFailure(a.Name(), a.Next, ctx, w, r)
	}

	if !firstInSearchPath(state.Name(), searchpath) {
		return plugin.NextOrFailure(a.Name(), a.Next, ctx, w, r)
	}

	origQName := state.QName()

	// Establish base name of the query. I.e what was originally asked.
	base, err := dnsutil.TrimZone(state.QName(), searchpath[0])
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	firstReply := new(dns.Msg)
	firstRcode := 0
	var firstErr error

	ar := r.Copy()
	// Walk the search path and see if we can get a non-nxdomain - if they all fail we return the first
	// query we've done and return that as-is. This means the client will do the search path walk again...
	for i, s := range searchpath {
		newQName := base + "." + s
		ar.Question[0].Name = newQName
		nw := nonwriter.New(w)

		rcode, err := plugin.NextOrFailure(a.Name(), a.Next, ctx, nw, ar)
		if err != nil {
			// Return now - not sure if this is the best. We should also check if the write has happened.
			return rcode, err
		}
		if i == 0 {
			firstReply = nw.Msg
			firstRcode = rcode
			firstErr = err
		}

		if !plugin.ClientWrite(rcode) {
			continue
		}

		if nw.Msg.Rcode == dns.RcodeNameError {
			continue
		}

		msg := nw.Msg
		cnamer(msg, origQName)

		// Write whatever non-nxdomain answer we've found.
		w.WriteMsg(msg)
		AutoPathCount.WithLabelValues().Add(1)
		return rcode, err

	}
	if plugin.ClientWrite(firstRcode) {
		w.WriteMsg(firstReply)
	}
	return firstRcode, firstErr
}

// Name implements the Handler interface.
func (a *AutoPath) Name() string { return "autopath" }

// firstInSearchPath checks if name is equal to are a sibling of the first element in the search path.
func firstInSearchPath(name string, searchpath []string) bool {
	if name == searchpath[0] {
		return true
	}
	if dns.IsSubDomain(searchpath[0], name) {
		return true
	}
	return false
}
