// Package rewrite is plugin for rewriting requests internally to something different.
package rewrite

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// edns0LocalRule is a rewrite rule for EDNS0_LOCAL options.
type edns0LocalRule struct {
	mode   string
	action string
	code   uint16
	data   []byte
}

// edns0VariableRule is a rewrite rule for EDNS0_LOCAL options with variable.
type edns0VariableRule struct {
	mode     string
	action   string
	code     uint16
	variable string
}

// ends0NsidRule is a rewrite rule for EDNS0_NSID options.
type edns0NsidRule struct {
	mode   string
	action string
}

// setupEdns0Opt will retrieve the EDNS0 OPT or create it if it does not exist.
func setupEdns0Opt(r *dns.Msg) *dns.OPT {
	o := r.IsEdns0()
	if o == nil {
		r.SetEdns0(4096, false)
		o = r.IsEdns0()
	}
	return o
}

// Rewrite will alter the request EDNS0 NSID option
func (rule *edns0NsidRule) Rewrite(state request.Request) Result {
	o := setupEdns0Opt(state.Req)

	for _, s := range o.Option {
		if e, ok := s.(*dns.EDNS0_NSID); ok {
			if rule.action == Replace || rule.action == Set {
				e.Nsid = "" // make sure it is empty for request
				return RewriteDone
			}
		}
	}

	// add option if not found
	if rule.action == Append || rule.action == Set {
		o.Option = append(o.Option, &dns.EDNS0_NSID{Code: dns.EDNS0NSID, Nsid: ""})
		return RewriteDone
	}

	return RewriteIgnored
}

// Mode returns the processing mode.
func (rule *edns0NsidRule) Mode() string { return rule.mode }

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *edns0NsidRule) GetResponseRule() ResponseRule { return ResponseRule{} }

// Rewrite will alter the request EDNS0 local options.
func (rule *edns0LocalRule) Rewrite(state request.Request) Result {
	o := setupEdns0Opt(state.Req)

	for _, s := range o.Option {
		if e, ok := s.(*dns.EDNS0_LOCAL); ok {
			if rule.code == e.Code {
				if rule.action == Replace || rule.action == Set {
					e.Data = rule.data
					return RewriteDone
				}
			}
		}
	}

	// add option if not found
	if rule.action == Append || rule.action == Set {
		o.Option = append(o.Option, &dns.EDNS0_LOCAL{Code: rule.code, Data: rule.data})
		return RewriteDone
	}

	return RewriteIgnored
}

// Mode returns the processing mode.
func (rule *edns0LocalRule) Mode() string { return rule.mode }

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *edns0LocalRule) GetResponseRule() ResponseRule { return ResponseRule{} }

// newEdns0Rule creates an EDNS0 rule of the appropriate type based on the args
func newEdns0Rule(mode string, args ...string) (Rule, error) {
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
		// Check for variable option.
		if strings.HasPrefix(args[3], "{") && strings.HasSuffix(args[3], "}") {
			return newEdns0VariableRule(mode, action, args[2], args[3])
		}
		return newEdns0LocalRule(mode, action, args[2], args[3])
	case "nsid":
		if len(args) != 2 {
			return nil, fmt.Errorf("EDNS0 NSID rules do not accept args")
		}
		return &edns0NsidRule{mode: mode, action: action}, nil
	case "subnet":
		if len(args) != 4 {
			return nil, fmt.Errorf("EDNS0 subnet rules require exactly three args")
		}
		return newEdns0SubnetRule(mode, action, args[2], args[3])
	default:
		return nil, fmt.Errorf("invalid rule type %q", ruleType)
	}
}

func newEdns0LocalRule(mode, action, code, data string) (*edns0LocalRule, error) {
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
	return &edns0LocalRule{mode: mode, action: action, code: uint16(c), data: decoded}, nil
}

// newEdns0VariableRule creates an EDNS0 rule that handles variable substitution
func newEdns0VariableRule(mode, action, code, variable string) (*edns0VariableRule, error) {
	c, err := strconv.ParseUint(code, 0, 16)
	if err != nil {
		return nil, err
	}
	//Validate
	if !isValidVariable(variable) {
		return nil, fmt.Errorf("unsupported variable name %q", variable)
	}
	return &edns0VariableRule{mode: mode, action: action, code: uint16(c), variable: variable}, nil
}

// ruleData returns the data specified by the variable.
func (rule *edns0VariableRule) ruleData(state request.Request) ([]byte, error) {

	switch rule.variable {
	case queryName:
		return []byte(state.QName()), nil

	case queryType:
		return uint16ToWire(state.QType()), nil

	case clientIP:
		return ipToWire(state.Family(), state.IP())

	case serverIP:
		return ipToWire(state.Family(), state.LocalIP())

	case clientPort:
		return portToWire(state.Port())

	case serverPort:
		return portToWire(state.LocalPort())

	case protocol:
		return []byte(state.Proto()), nil
	}

	return nil, fmt.Errorf("unable to extract data for variable %s", rule.variable)
}

// Rewrite will alter the request EDNS0 local options with specified variables.
func (rule *edns0VariableRule) Rewrite(state request.Request) Result {
	data, err := rule.ruleData(state)
	if err != nil || data == nil {
		return RewriteIgnored
	}

	o := setupEdns0Opt(state.Req)
	for _, s := range o.Option {
		if e, ok := s.(*dns.EDNS0_LOCAL); ok {
			if rule.code == e.Code {
				if rule.action == Replace || rule.action == Set {
					e.Data = data
					return RewriteDone
				}
				return RewriteIgnored
			}
		}
	}

	// add option if not found
	if rule.action == Append || rule.action == Set {
		o.Option = append(o.Option, &dns.EDNS0_LOCAL{Code: rule.code, Data: data})
		return RewriteDone
	}

	return RewriteIgnored
}

// Mode returns the processing mode.
func (rule *edns0VariableRule) Mode() string { return rule.mode }

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *edns0VariableRule) GetResponseRule() ResponseRule { return ResponseRule{} }

func isValidVariable(variable string) bool {
	switch variable {
	case
		queryName,
		queryType,
		clientIP,
		clientPort,
		protocol,
		serverIP,
		serverPort:
		return true
	}
	return false
}

// ends0SubnetRule is a rewrite rule for EDNS0 subnet options
type edns0SubnetRule struct {
	mode         string
	v4BitMaskLen uint8
	v6BitMaskLen uint8
	action       string
}

func newEdns0SubnetRule(mode, action, v4BitMaskLen, v6BitMaskLen string) (*edns0SubnetRule, error) {
	v4Len, err := strconv.ParseUint(v4BitMaskLen, 0, 16)
	if err != nil {
		return nil, err
	}
	// validate V4 length
	if v4Len > net.IPv4len*8 {
		return nil, fmt.Errorf("invalid IPv4 bit mask length %d", v4Len)
	}

	v6Len, err := strconv.ParseUint(v6BitMaskLen, 0, 16)
	if err != nil {
		return nil, err
	}
	// validate V6 length
	if v6Len > net.IPv6len*8 {
		return nil, fmt.Errorf("invalid IPv6 bit mask length %d", v6Len)
	}

	return &edns0SubnetRule{mode: mode, action: action,
		v4BitMaskLen: uint8(v4Len), v6BitMaskLen: uint8(v6Len)}, nil
}

// fillEcsData sets the subnet data into the ecs option
func (rule *edns0SubnetRule) fillEcsData(state request.Request, ecs *dns.EDNS0_SUBNET) error {
	family := state.Family()
	if (family != 1) && (family != 2) {
		return fmt.Errorf("unable to fill data for EDNS0 subnet due to invalid IP family")
	}

	ecs.Family = uint16(family)
	ecs.SourceScope = 0

	ipAddr := state.IP()
	switch family {
	case 1:
		ipv4Mask := net.CIDRMask(int(rule.v4BitMaskLen), 32)
		ipv4Addr := net.ParseIP(ipAddr)
		ecs.SourceNetmask = rule.v4BitMaskLen
		ecs.Address = ipv4Addr.Mask(ipv4Mask).To4()
	case 2:
		ipv6Mask := net.CIDRMask(int(rule.v6BitMaskLen), 128)
		ipv6Addr := net.ParseIP(ipAddr)
		ecs.SourceNetmask = rule.v6BitMaskLen
		ecs.Address = ipv6Addr.Mask(ipv6Mask).To16()
	}
	return nil
}

// Rewrite will alter the request EDNS0 subnet option.
func (rule *edns0SubnetRule) Rewrite(state request.Request) Result {
	o := setupEdns0Opt(state.Req)

	for _, s := range o.Option {
		if e, ok := s.(*dns.EDNS0_SUBNET); ok {
			if rule.action == Replace || rule.action == Set {
				if rule.fillEcsData(state, e) == nil {
					return RewriteDone
				}
			}
			return RewriteIgnored
		}
	}

	// add option if not found
	if rule.action == Append || rule.action == Set {
		opt := &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET}
		if rule.fillEcsData(state, opt) == nil {
			o.Option = append(o.Option, opt)
			return RewriteDone
		}
	}

	return RewriteIgnored
}

// Mode returns the processing mode
func (rule *edns0SubnetRule) Mode() string { return rule.mode }

// GetResponseRule return a rule to rewrite the response with. Currently not implemented.
func (rule *edns0SubnetRule) GetResponseRule() ResponseRule { return ResponseRule{} }

// These are all defined actions.
const (
	Replace = "replace"
	Set     = "set"
	Append  = "append"
)

// Supported local EDNS0 variables
const (
	queryName  = "{qname}"
	queryType  = "{qtype}"
	clientIP   = "{client_ip}"
	clientPort = "{client_port}"
	protocol   = "{protocol}"
	serverIP   = "{server_ip}"
	serverPort = "{server_port}"
)
