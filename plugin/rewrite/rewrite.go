package rewrite

import (
	"context"
	"fmt"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Result is the result of a rewrite
type Result int

const (
	// RewriteIgnored is returned when rewrite is not done on request.
	RewriteIgnored Result = iota
	// RewriteDone is returned when rewrite is done on request.
	RewriteDone
)

// These are defined processing mode.
const (
	// Processing should stop after completing this rule
	Stop = "stop"
	// Processing should continue to next rule
	Continue = "continue"
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
	state := request.Request{W: w, Req: r}

	for _, rule := range rw.Rules {
		switch result := rule.Rewrite(state); result {
		case RewriteDone:
			respRule := rule.GetResponseRule()
			if respRule.Active == true {
				wr.ResponseRewrite = true
				wr.ResponseRules = append(wr.ResponseRules, respRule)
			}
			if rule.Mode() == Stop {
				if rw.noRevert {
					return plugin.NextOrFailure(rw.Name(), rw.Next, ctx, w, r)
				}
				return plugin.NextOrFailure(rw.Name(), rw.Next, ctx, wr, r)
			}
		case RewriteIgnored:
			break
		}
	}
	if rw.noRevert || len(wr.ResponseRules) == 0 {
		return plugin.NextOrFailure(rw.Name(), rw.Next, ctx, w, r)
	}
	return plugin.NextOrFailure(rw.Name(), rw.Next, ctx, wr, r)
}

// Name implements the Handler interface.
func (rw Rewrite) Name() string { return "rewrite" }

// Rule describes a rewrite rule.
type Rule interface {
	// Rewrite rewrites the current request.
	Rewrite(state request.Request) Result
	// Mode returns the processing mode stop or continue.
	Mode() string
	// GetResponseRule returns the rule to rewrite response with, if any.
	GetResponseRule() ResponseRule
}

func newRule(args ...string) (Rule, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no rule type specified for rewrite")
	}

	arg0 := strings.ToLower(args[0])
	var ruleType string
	var expectNumArgs, startArg int
	mode := Stop
	switch arg0 {
	case Continue:
		mode = Continue
		ruleType = strings.ToLower(args[1])
		expectNumArgs = len(args) - 1
		startArg = 2
	case Stop:
		ruleType = strings.ToLower(args[1])
		expectNumArgs = len(args) - 1
		startArg = 2
	default:
		// for backward compatibility
		ruleType = arg0
		expectNumArgs = len(args)
		startArg = 1
	}

	if ruleType == "answer" {
		return nil, fmt.Errorf("response rewrites must begin with a name rule")
	}

	if ruleType != "edns0" && ruleType != "name" && expectNumArgs != 3 {
		return nil, fmt.Errorf("%s rules must have exactly two arguments", ruleType)
	}

	switch ruleType {
	case "name":
		return newNameRule(mode, args[startArg:]...)
	case "class":
		return newClassRule(mode, args[startArg:]...)
	case "type":
		return newTypeRule(mode, args[startArg:]...)
	case "edns0":
		return newEdns0Rule(mode, args[startArg:]...)
	default:
		return nil, fmt.Errorf("invalid rule type %q", args[0])
	}
}
