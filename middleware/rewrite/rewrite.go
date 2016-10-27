// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

import (
	"strings"

	"github.com/miekg/coredns/middleware"

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

// Rewrite is middleware to rewrite requests internally before being handled.
type Rewrite struct {
	Next     middleware.Handler
	Rules    []Rule
	noRevert bool
}

// ServeDNS implements the middleware.Handler interface.
func (rw Rewrite) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	wr := NewResponseReverter(w, r)
	for _, rule := range rw.Rules {
		switch result := rule.Rewrite(r); result {
		case RewriteDone:
			if rw.noRevert {
				return rw.Next.ServeDNS(ctx, w, r)
			}
			return rw.Next.ServeDNS(ctx, wr, r)
		case RewriteIgnored:
			break
		case RewriteStatus:
			// only valid for complex rules.
			// if cRule, ok := rule.(*ComplexRule); ok && cRule.Status != 0 {
			// return cRule.Status, nil
			// }
		}
	}
	return rw.Next.ServeDNS(ctx, w, r)
}

// Name implements the Handler interface.
func (rw Rewrite) Name() string { return "rewrite" }

// Rule describes an internal location rewrite rule.
type Rule interface {
	// Rewrite rewrites the internal location of the current request.
	Rewrite(*dns.Msg) Result
}

// SimpleRule is a simple rewrite rule. If the From and To look like a type
// the type of the request is rewritten, otherwise the name is.
// Note: TSIG signed requests will be invalid.
type SimpleRule struct {
	From, To           string
	fromType, toType   uint16
	fromClass, toClass uint16
}

// NewSimpleRule creates a new Simple Rule
func NewSimpleRule(from, to string) SimpleRule {
	tpf := dns.StringToType[from]
	tpt := dns.StringToType[to]

	// ANY is both a type and class, ANY class rewritting is way more less frequent
	// so we default to ANY as a type.
	clf := dns.StringToClass[from]
	clt := dns.StringToClass[to]
	if from == "ANY" {
		clf = 0
		clt = 0
	}

	// It's only a type/class if uppercase is used.
	if from != strings.ToUpper(from) {
		tpf = 0
		clf = 0
		from = middleware.Name(from).Normalize()
	}
	if to != strings.ToUpper(to) {
		tpt = 0
		clt = 0
		to = middleware.Name(to).Normalize()
	}
	return SimpleRule{From: from, To: to, fromType: tpf, toType: tpt, fromClass: clf, toClass: clt}
}

// Rewrite rewrites the the current request.
func (s SimpleRule) Rewrite(r *dns.Msg) Result {
	if s.fromType > 0 && s.toType > 0 {
		if r.Question[0].Qtype == s.fromType {
			r.Question[0].Qtype = s.toType
			return RewriteDone
		}
	}

	if s.fromClass > 0 && s.toClass > 0 {
		if r.Question[0].Qclass == s.fromClass {
			r.Question[0].Qclass = s.toClass
			return RewriteDone
		}
	}

	if s.From == r.Question[0].Name {
		r.Question[0].Name = s.To
		return RewriteDone
	}
	return RewriteIgnored
}
