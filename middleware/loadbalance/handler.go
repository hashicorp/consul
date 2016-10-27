// Package loadbalance is middleware for rewriting responses to do "load balancing"
package loadbalance

import (
	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// RoundRobin is middleware to rewrite responses for "load balancing".
type RoundRobin struct {
	Next middleware.Handler
}

// ServeDNS implements the middleware.Handler interface.
func (rr RoundRobin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	wrr := &RoundRobinResponseWriter{w}
	return rr.Next.ServeDNS(ctx, wrr, r)
}

// Name implements the Handler interface.
func (rr RoundRobin) Name() string { return "loadbalance" }
