// Package fuzz contains functions that enable fuzzing of plugins.
package fuzz

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// Do will fuzz p - used by gofuzz. See Makefile.fuzz for comments and context.
func Do(p plugin.Handler, data []byte) int {
	ctx := context.TODO()
	r := new(dns.Msg)
	if err := r.Unpack(data); err != nil {
		return 0 // plugin will never be called when this happens.
	}
	// If the data unpack into a dns msg, but does not have a proper question section discard it.
	// The server parts make sure this is true before calling the plugins; mimic this behavior.
	if len(r.Question) == 0 {
		return 0
	}

	if _, err := p.ServeDNS(ctx, &test.ResponseWriter{}, r); err != nil {
		return 1
	}

	return 0
}
