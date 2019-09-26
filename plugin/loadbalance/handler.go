// Package loadbalance is a plugin for rewriting responses to do "load balancing"
package loadbalance

import (
	"context"

	"github.com/coredns/coredns/plugin"

	"github.com/miekg/dns"
)

// RoundRobin is a plugin to rewrite responses for "load balancing".
type RoundRobin struct {
	Next plugin.Handler
}

// ServeDNS implements the plugin.Handler interface.
func (rr RoundRobin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	wrr := &RoundRobinResponseWriter{w}
	return plugin.NextOrFailure(rr.Name(), rr.Next, ctx, wrr, r)
}

// Name implements the Handler interface.
func (rr RoundRobin) Name() string { return "loadbalance" }
