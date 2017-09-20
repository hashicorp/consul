// Package rewrite is plugin for rewriting requests internally to something different.
package rewrite

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

// typeRule is a type rewrite rule.
type typeRule struct {
	fromType, toType uint16
}

func newTypeRule(fromS, toS string) (Rule, error) {
	var from, to uint16
	var ok bool
	if from, ok = dns.StringToType[strings.ToUpper(fromS)]; !ok {
		return nil, fmt.Errorf("invalid type %q", strings.ToUpper(fromS))
	}
	if to, ok = dns.StringToType[strings.ToUpper(toS)]; !ok {
		return nil, fmt.Errorf("invalid type %q", strings.ToUpper(toS))
	}
	return &typeRule{fromType: from, toType: to}, nil
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
	return Stop
}
