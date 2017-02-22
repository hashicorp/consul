// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

import (
	"strings"

	"github.com/coredns/coredns/middleware"
	"github.com/miekg/dns"
)

// NameRule is a name rewrite rule.
type NameRule struct {
	From, To string
}

// Initializer
func (rule NameRule) New(args ...string) Rule {
	from, to := args[0], strings.Join(args[1:], " ")
	return &NameRule{middleware.Name(from).Normalize(), middleware.Name(to).Normalize()}
}

// Rewrite rewrites the the current request.
func (rule NameRule) Rewrite(r *dns.Msg) Result {
	if rule.From == r.Question[0].Name {
		r.Question[0].Name = rule.To
		return RewriteDone
	}
	return RewriteIgnored
}
