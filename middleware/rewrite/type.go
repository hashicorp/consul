// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

import (
	"strings"

	"github.com/miekg/dns"
)

// TypeRule is a type rewrite rule.
type TypeRule struct {
	fromType, toType uint16
}

// Initializer
func (rule TypeRule) New(args ...string) Rule {
	from, to := args[0], strings.Join(args[1:], " ")
	return &TypeRule{dns.StringToType[from], dns.StringToType[to]}
}

// Rewrite rewrites the the current request.
func (rule TypeRule) Rewrite(r *dns.Msg) Result {
	if rule.fromType > 0 && rule.toType > 0 {
		if r.Question[0].Qtype == rule.fromType {
			r.Question[0].Qtype = rule.toType
			return RewriteDone
		}
	}
	return RewriteIgnored
}
