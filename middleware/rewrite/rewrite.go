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

// ServeHTTP implements the middleware.Handler interface.
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

/*
// ComplexRule is a rewrite rule based on a regular expression
type ComplexRule struct {
	// Path base. Request to this path and subpaths will be rewritten
	Base string

	// Path to rewrite to
	To string

	// If set, neither performs rewrite nor proceeds
	// with request. Only returns code.
	Status int

	// Extensions to filter by
	Exts []string

	// Rewrite conditions
	Ifs []If

	*regexp.Regexp
}

// NewComplexRule creates a new RegexpRule. It returns an error if regexp
// pattern (pattern) or extensions (ext) are invalid.
func NewComplexRule(base, pattern, to string, status int, ext []string, ifs []If) (*ComplexRule, error) {
	// validate regexp if present
	var r *regexp.Regexp
	if pattern != "" {
		var err error
		r, err = regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
	}

	// validate extensions if present
	for _, v := range ext {
		if len(v) < 2 || (len(v) < 3 && v[0] == '!') {
			// check if no extension is specified
			if v != "/" && v != "!/" {
				return nil, fmt.Errorf("invalid extension %v", v)
			}
		}
	}

	return &ComplexRule{
		Base:   base,
		To:     to,
		Status: status,
		Exts:   ext,
		Ifs:    ifs,
		Regexp: r,
	}, nil
}

// Rewrite rewrites the internal location of the current request.
func (r *ComplexRule) Rewrite(req *dns.Msg) (re Result) {
	rPath := req.URL.Path
	replacer := newReplacer(req)

	// validate base
	if !middleware.Path(rPath).Matches(r.Base) {
		return
	}

	// validate extensions
	if !r.matchExt(rPath) {
		return
	}

	// validate regexp if present
	if r.Regexp != nil {
		// include trailing slash in regexp if present
		start := len(r.Base)
		if strings.HasSuffix(r.Base, "/") {
			start--
		}

		matches := r.FindStringSubmatch(rPath[start:])
		switch len(matches) {
		case 0:
			// no match
			return
		default:
			// set regexp match variables {1}, {2} ...
			for i := 1; i < len(matches); i++ {
				replacer.Set(fmt.Sprint(i), matches[i])
			}
		}
	}

	// validate rewrite conditions
	for _, i := range r.Ifs {
		if !i.True(req) {
			return
		}
	}

	// if status is present, stop rewrite and return it.
	if r.Status != 0 {
		return RewriteStatus
	}

	// attempt rewrite
	return To(fs, req, r.To, replacer)
}

// matchExt matches rPath against registered file extensions.
// Returns true if a match is found and false otherwise.
func (r *ComplexRule) matchExt(rPath string) bool {
	f := filepath.Base(rPath)
	ext := path.Ext(f)
	if ext == "" {
		ext = "/"
	}

	mustUse := false
	for _, v := range r.Exts {
		use := true
		if v[0] == '!' {
			use = false
			v = v[1:]
		}

		if use {
			mustUse = true
		}

		if ext == v {
			return use
		}
	}

	if mustUse {
		return false
	}
	return true
}
*/
