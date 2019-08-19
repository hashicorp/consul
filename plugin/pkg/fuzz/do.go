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
	ret := 1
	r := new(dns.Msg)
	if err := r.Unpack(data); err != nil {
		ret = 0
	}

	if _, err := p.ServeDNS(ctx, &test.ResponseWriter{}, r); err != nil {
		ret = 1
	}

	return ret
}
