// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

// edns0LocalRule is a rewrite rule for EDNS0_LOCAL options
type edns0LocalRule struct {
	action string
	code   uint16
	data   []byte
}

// ends0NsidRule is a rewrite rule for EDNS0_NSID options
type edns0NsidRule struct {
	action string
}

// setupEdns0Opt will retrieve the EDNS0 OPT or create it if it does not exist
func setupEdns0Opt(r *dns.Msg) *dns.OPT {
	o := r.IsEdns0()
	if o == nil {
		r.SetEdns0(4096, true)
		o = r.IsEdns0()
	}
	return o
}

// Rewrite will alter the request EDNS0 NSID option
func (rule *edns0NsidRule) Rewrite(r *dns.Msg) Result {
	result := RewriteIgnored
	o := setupEdns0Opt(r)
	found := false
Option:
	for _, s := range o.Option {
		switch e := s.(type) {
		case *dns.EDNS0_NSID:
			if rule.action == Replace || rule.action == Set {
				e.Nsid = "" // make sure it is empty for request
				result = RewriteDone
			}
			found = true
			break Option
		}
	}

	// add option if not found
	if !found && (rule.action == Append || rule.action == Set) {
		o.SetDo()
		o.Option = append(o.Option, &dns.EDNS0_NSID{Code: dns.EDNS0NSID, Nsid: ""})
		result = RewriteDone
	}

	return result
}

// Rewrite will alter the request EDNS0 local options
func (rule *edns0LocalRule) Rewrite(r *dns.Msg) Result {
	result := RewriteIgnored
	o := setupEdns0Opt(r)
	found := false
	for _, s := range o.Option {
		switch e := s.(type) {
		case *dns.EDNS0_LOCAL:
			if rule.code == e.Code {
				if rule.action == Replace || rule.action == Set {
					e.Data = rule.data
					result = RewriteDone
				}
				found = true
				break
			}
		}
	}

	// add option if not found
	if !found && (rule.action == Append || rule.action == Set) {
		o.SetDo()
		var opt dns.EDNS0_LOCAL
		opt.Code = rule.code
		opt.Data = rule.data
		o.Option = append(o.Option, &opt)
		result = RewriteDone
	}

	return result
}

// newEdns0Rule creates an EDNS0 rule of the appropriate type based on the args
func newEdns0Rule(args ...string) (Rule, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("too few arguments for an EDNS0 rule")
	}

	ruleType := strings.ToLower(args[0])
	action := strings.ToLower(args[1])
	switch action {
	case Append:
	case Replace:
	case Set:
	default:
		return nil, fmt.Errorf("invalid action: %q", action)
	}

	switch ruleType {
	case "local":
		if len(args) != 4 {
			return nil, fmt.Errorf("EDNS0 local rules require exactly three args")
		}
		return newEdns0LocalRule(action, args[2], args[3])
	case "nsid":
		if len(args) != 2 {
			return nil, fmt.Errorf("EDNS0 NSID rules do not accept args")
		}
		return &edns0NsidRule{action: action}, nil
	default:
		return nil, fmt.Errorf("invalid rule type %q", ruleType)
	}
}

func newEdns0LocalRule(action, code, data string) (*edns0LocalRule, error) {
	c, err := strconv.ParseUint(code, 0, 16)
	if err != nil {
		return nil, err
	}

	decoded := []byte(data)
	if strings.HasPrefix(data, "0x") {
		decoded, err = hex.DecodeString(data[2:])
		if err != nil {
			return nil, err
		}
	}

	return &edns0LocalRule{action: action, code: uint16(c), data: decoded}, nil
}

// These are all defined actions.
const (
	Replace = "replace"
	Set     = "set"
	Append  = "append"
)
