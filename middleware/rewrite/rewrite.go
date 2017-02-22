// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

import (
	"github.com/coredns/coredns/middleware"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Result is the result of a rewrite
type Result int

const (
	// RewriteIgnored is returned when rewrite is not done on request.
	RewriteIgnored Result = iota
	// RewriteDone is returned when rewrite is done on request.
	RewriteDone
	// RewriteStatus is returned when rewrite is not needed and status code should be set
	// for the request.
	RewriteStatus
)

// Rewrite is middleware to rewrite requests internally before being handled.
type Rewrite struct {
	Next     middleware.Handler
	Rules    []Rule
	noRevert bool
}

// ServeDNS implements the middleware.Handler interface.
func (rw Rewrite) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	wr := NewResponseReverter(w, r)
	for _, rule := range rw.Rules {
		switch result := rule.Rewrite(r); result {
		case RewriteDone:
			if rw.noRevert {
				return middleware.NextOrFailure(rw.Name(), rw.Next, ctx, w, r)
			}
			return middleware.NextOrFailure(rw.Name(), rw.Next, ctx, wr, r)
		case RewriteIgnored:
			break
		case RewriteStatus:
			// only valid for complex rules.
			// if cRule, ok := rule.(*ComplexRule); ok && cRule.Status != 0 {
			// return cRule.Status, nil
			// }
		}
	}
	return middleware.NextOrFailure(rw.Name(), rw.Next, ctx, w, r)
}

// Name implements the Handler interface.
func (rw Rewrite) Name() string { return "rewrite" }

// Rule describes an internal location rewrite rule.
type Rule interface {
	// Rewrite rewrites the internal location of the current request.
	Rewrite(*dns.Msg) Result
	// New returns a new rule.
	New(...string) Rule
}
