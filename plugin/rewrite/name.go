package rewrite

import (
	"fmt"
	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
	"regexp"
	"strconv"
	"strings"
)

type nameRule struct {
	NextAction string
	From       string
	To         string
}

type prefixNameRule struct {
	NextAction  string
	Prefix      string
	Replacement string
}

type suffixNameRule struct {
	NextAction  string
	Suffix      string
	Replacement string
}

type substringNameRule struct {
	NextAction  string
	Substring   string
	Replacement string
}

type regexNameRule struct {
	NextAction  string
	Pattern     *regexp.Regexp
	Replacement string
	ResponseRule
}

const (
	// ExactMatch matches only on exact match of the name in the question section of a request
	ExactMatch = "exact"
	// PrefixMatch matches when the name begins with the matching string
	PrefixMatch = "prefix"
	// SuffixMatch matches when the name ends with the matching string
	SuffixMatch = "suffix"
	// SubstringMatch matches on partial match of the name in the question section of a request
	SubstringMatch = "substring"
	// RegexMatch matches when the name in the question section of a request matches a regular expression
	RegexMatch = "regex"
)

// Rewrite rewrites the current request based upon exact match of the name
// in the question section of the request
func (rule *nameRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	if rule.From == r.Question[0].Name {
		r.Question[0].Name = rule.To
		return RewriteDone
	}
	return RewriteIgnored
}

// Rewrite rewrites the current request when the name begins with the matching string
func (rule *prefixNameRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	if strings.HasPrefix(r.Question[0].Name, rule.Prefix) {
		r.Question[0].Name = rule.Replacement + strings.TrimLeft(r.Question[0].Name, rule.Prefix)
		return RewriteDone
	}
	return RewriteIgnored
}

// Rewrite rewrites the current request when the name ends with the matching string
func (rule *suffixNameRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	if strings.HasSuffix(r.Question[0].Name, rule.Suffix) {
		r.Question[0].Name = strings.TrimRight(r.Question[0].Name, rule.Suffix) + rule.Replacement
		return RewriteDone
	}
	return RewriteIgnored
}

// Rewrite rewrites the current request based upon partial match of the
// name in the question section of the request
func (rule *substringNameRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	if strings.Contains(r.Question[0].Name, rule.Substring) {
		r.Question[0].Name = strings.Replace(r.Question[0].Name, rule.Substring, rule.Replacement, -1)
		return RewriteDone
	}
	return RewriteIgnored
}

// Rewrite rewrites the current request when the name in the question
// section of the request matches a regular expression
func (rule *regexNameRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	regexGroups := rule.Pattern.FindStringSubmatch(r.Question[0].Name)
	if len(regexGroups) == 0 {
		return RewriteIgnored
	}
	s := rule.Replacement
	for groupIndex, groupValue := range regexGroups {
		groupIndexStr := "{" + strconv.Itoa(groupIndex) + "}"
		if strings.Contains(s, groupIndexStr) {
			s = strings.Replace(s, groupIndexStr, groupValue, -1)
		}
	}
	r.Question[0].Name = s
	return RewriteDone
}

// newNameRule creates a name matching rule based on exact, partial, or regex match
func newNameRule(nextAction string, args ...string) (Rule, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("too few arguments for a name rule")
	}
	if len(args) == 3 {
		switch strings.ToLower(args[0]) {
		case ExactMatch:
			return &nameRule{nextAction, plugin.Name(args[1]).Normalize(), plugin.Name(args[2]).Normalize()}, nil
		case PrefixMatch:
			return &prefixNameRule{nextAction, plugin.Name(args[1]).Normalize(), plugin.Name(args[2]).Normalize()}, nil
		case SuffixMatch:
			return &suffixNameRule{nextAction, plugin.Name(args[1]).Normalize(), plugin.Name(args[2]).Normalize()}, nil
		case SubstringMatch:
			return &substringNameRule{nextAction, plugin.Name(args[1]).Normalize(), plugin.Name(args[2]).Normalize()}, nil
		case RegexMatch:
			regexPattern, err := regexp.Compile(args[1])
			if err != nil {
				return nil, fmt.Errorf("Invalid regex pattern in a name rule: %s", args[1])
			}
			return &regexNameRule{nextAction, regexPattern, plugin.Name(args[2]).Normalize(), ResponseRule{}}, nil
		default:
			return nil, fmt.Errorf("A name rule supports only exact, prefix, suffix, substring, and regex name matching")
		}
	}
	if len(args) == 7 {
		if strings.ToLower(args[0]) == RegexMatch {
			if args[3] != "answer" {
				return nil, fmt.Errorf("exceeded the number of arguments for a regex name rule")
			}
			switch strings.ToLower(args[4]) {
			case "name":
			default:
				return nil, fmt.Errorf("exceeded the number of arguments for a regex name rule")
			}
			regexPattern, err := regexp.Compile(args[1])
			if err != nil {
				return nil, fmt.Errorf("Invalid regex pattern in a name rule: %s", args)
			}
			responseRegexPattern, err := regexp.Compile(args[5])
			if err != nil {
				return nil, fmt.Errorf("Invalid regex pattern in a name rule: %s", args)
			}
			return &regexNameRule{
				nextAction,
				regexPattern,
				plugin.Name(args[2]).Normalize(),
				ResponseRule{
					Active:      true,
					Pattern:     responseRegexPattern,
					Replacement: plugin.Name(args[6]).Normalize(),
				},
			}, nil
		}
		return nil, fmt.Errorf("the rewrite of response is supported only for name regex rule")
	}
	if len(args) > 3 && len(args) != 7 {
		return nil, fmt.Errorf("exceeded the number of arguments for a name rule")
	}
	return &nameRule{nextAction, plugin.Name(args[0]).Normalize(), plugin.Name(args[1]).Normalize()}, nil
}

// Mode returns the processing nextAction
func (rule *nameRule) Mode() string {
	return rule.NextAction
}

func (rule *prefixNameRule) Mode() string {
	return rule.NextAction
}

func (rule *suffixNameRule) Mode() string {
	return rule.NextAction
}

func (rule *substringNameRule) Mode() string {
	return rule.NextAction
}

func (rule *regexNameRule) Mode() string {
	return rule.NextAction
}

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *nameRule) GetResponseRule() ResponseRule {
	return ResponseRule{}
}

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *prefixNameRule) GetResponseRule() ResponseRule {
	return ResponseRule{}
}

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *suffixNameRule) GetResponseRule() ResponseRule {
	return ResponseRule{}
}

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *substringNameRule) GetResponseRule() ResponseRule {
	return ResponseRule{}
}

// GetResponseRule return a rule to rewrite the response with.
func (rule *regexNameRule) GetResponseRule() ResponseRule {
	return rule.ResponseRule
}
