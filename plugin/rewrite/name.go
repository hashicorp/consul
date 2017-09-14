package rewrite

import (
	"github.com/coredns/coredns/plugin"

	"github.com/miekg/dns"
)

type nameRule struct {
	From, To string
}

func newNameRule(from, to string) (Rule, error) {
	return &nameRule{plugin.Name(from).Normalize(), plugin.Name(to).Normalize()}, nil
}

// Rewrite rewrites the the current request.
func (rule *nameRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	if rule.From == r.Question[0].Name {
		r.Question[0].Name = rule.To
		return RewriteDone
	}
	return RewriteIgnored
}
