// Package rewrite is plugin for rewriting requests internally to something different.
package rewrite

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

// typeRule is a type rewrite rule.
type typeRule struct {
	fromType   uint16
	toType     uint16
	nextAction string
}

func newTypeRule(nextAction string, args ...string) (Rule, error) {
	var from, to uint16
	var ok bool
	if from, ok = dns.StringToType[strings.ToUpper(args[0])]; !ok {
		return nil, fmt.Errorf("invalid type %q", strings.ToUpper(args[0]))
	}
	if to, ok = dns.StringToType[strings.ToUpper(args[1])]; !ok {
		return nil, fmt.Errorf("invalid type %q", strings.ToUpper(args[1]))
	}
	return &typeRule{from, to, nextAction}, nil
}

// Rewrite rewrites the the current request.
func (rule *typeRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	if rule.fromType > 0 && rule.toType > 0 {
		if r.Question[0].Qtype == rule.fromType {
			r.Question[0].Qtype = rule.toType
			return RewriteDone
		}
	}
	return RewriteIgnored
}

// Mode returns the processing mode
func (rule *typeRule) Mode() string {
	return rule.nextAction
}

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *typeRule) GetResponseRule() ResponseRule {
	return ResponseRule{}
}
