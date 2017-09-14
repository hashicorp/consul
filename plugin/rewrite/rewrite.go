package rewrite

import (
	"fmt"
	"strings"

	"github.com/coredns/coredns/plugin"

	"github.com/miekg/dns"

	"golang.org/x/net/context"
)

// Result is the result of a rewrite
type Result int

const (
	// RewriteIgnored is returned when rewrite is not done on request.
	RewriteIgnored Result = iota
	// RewriteDone is returned when rewrite is done on request.
	RewriteDone
	// RewriteStatus is returned when rewrite is not needed and status code should be set
	// for the request.
	RewriteStatus
)

// Rewrite is plugin to rewrite requests internally before being handled.
type Rewrite struct {
	Next     plugin.Handler
	Rules    []Rule
	noRevert bool
}

// ServeDNS implements the plugin.Handler interface.
func (rw Rewrite) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	wr := NewResponseReverter(w, r)
	for _, rule := range rw.Rules {
		switch result := rule.Rewrite(w, r); result {
		case RewriteDone:
			if rw.noRevert {
				return plugin.NextOrFailure(rw.Name(), rw.Next, ctx, w, r)
			}
			return plugin.NextOrFailure(rw.Name(), rw.Next, ctx, wr, r)
		case RewriteIgnored:
			break
		case RewriteStatus:
			// only valid for complex rules.
			// if cRule, ok := rule.(*ComplexRule); ok && cRule.Status != 0 {
			// return cRule.Status, nil
			// }
		}
	}
	return plugin.NextOrFailure(rw.Name(), rw.Next, ctx, w, r)
}

// Name implements the Handler interface.
func (rw Rewrite) Name() string { return "rewrite" }

// Rule describes a rewrite rule.
type Rule interface {
	// Rewrite rewrites the current request.
	Rewrite(dns.ResponseWriter, *dns.Msg) Result
}

func newRule(args ...string) (Rule, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no rule type specified for rewrite")
	}

	ruleType := strings.ToLower(args[0])
	if ruleType != "edns0" && len(args) != 3 {
		return nil, fmt.Errorf("%s rules must have exactly two arguments", ruleType)
	}
	switch ruleType {
	case "name":
		return newNameRule(args[1], args[2])
	case "class":
		return newClassRule(args[1], args[2])
	case "type":
		return newTypeRule(args[1], args[2])
	case "edns0":
		return newEdns0Rule(args[1:]...)
	default:
		return nil, fmt.Errorf("invalid rule type %q", args[0])
	}
}
