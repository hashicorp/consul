package rewrite

import (
	"github.com/coredns/coredns/middleware"

	"github.com/miekg/dns"
)

type nameRule struct {
	From, To string
}

func newNameRule(from, to string) (Rule, error) {
	return &nameRule{middleware.Name(from).Normalize(), middleware.Name(to).Normalize()}, nil
}

// Rewrite rewrites the the current request.
func (rule *nameRule) Rewrite(r *dns.Msg) Result {
	if rule.From == r.Question[0].Name {
		r.Question[0].Name = rule.To
		return RewriteDone
	}
	return RewriteIgnored
}
