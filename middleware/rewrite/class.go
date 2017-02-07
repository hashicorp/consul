// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

import (
	"strings"

	"github.com/miekg/dns"
)

// ClassRule is a class rewrite rule.
type ClassRule struct {
	fromClass, toClass uint16
}

// Initializer
func (rule ClassRule) New(args ...string) Rule {
	from, to := args[0], strings.Join(args[1:], " ")
	return &ClassRule{dns.StringToClass[from], dns.StringToClass[to]}

}

// Rewrite rewrites the the current request.
func (rule ClassRule) Rewrite(r *dns.Msg) Result {
	if rule.fromClass > 0 && rule.toClass > 0 {
		if r.Question[0].Qclass == rule.fromClass {
			r.Question[0].Qclass = rule.toClass
			return RewriteDone
		}
	}
	return RewriteIgnored
}
