package rewrite

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

type classRule struct {
	fromClass, toClass uint16
}

func newClassRule(fromS, toS string) (Rule, error) {
	var from, to uint16
	var ok bool
	if from, ok = dns.StringToClass[strings.ToUpper(fromS)]; !ok {
		return nil, fmt.Errorf("invalid class %q", strings.ToUpper(fromS))
	}
	if to, ok = dns.StringToClass[strings.ToUpper(toS)]; !ok {
		return nil, fmt.Errorf("invalid class %q", strings.ToUpper(toS))
	}
	return &classRule{fromClass: from, toClass: to}, nil
}

// Rewrite rewrites the the current request.
func (rule *classRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	if rule.fromClass > 0 && rule.toClass > 0 {
		if r.Question[0].Qclass == rule.fromClass {
			r.Question[0].Qclass = rule.toClass
			return RewriteDone
		}
	}
	return RewriteIgnored
}

// Mode returns the processing mode
func (rule *classRule) Mode() string {
	return Stop
}
