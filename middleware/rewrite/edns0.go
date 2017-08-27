// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

// edns0LocalRule is a rewrite rule for EDNS0_LOCAL options
type edns0LocalRule struct {
	action string
	code   uint16
	data   []byte
}

// edns0VariableRule is a rewrite rule for EDNS0_LOCAL options with variable
type edns0VariableRule struct {
	action   string
	code     uint16
	variable string
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
func (rule *edns0NsidRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
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
func (rule *edns0LocalRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
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
		//Check for variable option
		if strings.HasPrefix(args[3], "{") && strings.HasSuffix(args[3], "}") {
			return newEdns0VariableRule(action, args[2], args[3])
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

// newEdns0VariableRule creates an EDNS0 rule that handles variable substitution
func newEdns0VariableRule(action, code, variable string) (*edns0VariableRule, error) {
	c, err := strconv.ParseUint(code, 0, 16)
	if err != nil {
		return nil, err
	}
	//Validate
	if !isValidVariable(variable) {
		return nil, fmt.Errorf("unsupported variable name %q", variable)
	}
	return &edns0VariableRule{action: action, code: uint16(c), variable: variable}, nil
}

// ipToWire writes IP address to wire/binary format, 4 or 16 bytes depends on IPV4 or IPV6.
func (rule *edns0VariableRule) ipToWire(family int, ipAddr string) ([]byte, error) {

	switch family {
	case 1:
		return net.ParseIP(ipAddr).To4(), nil
	case 2:
		return net.ParseIP(ipAddr).To16(), nil
	}
	return nil, fmt.Errorf("Invalid IP address family (i.e. version) %d", family)
}

// uint16ToWire writes unit16 to wire/binary format
func (rule *edns0VariableRule) uint16ToWire(data uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(data))
	return buf
}

// portToWire writes port to wire/binary format, 2 bytes
func (rule *edns0VariableRule) portToWire(portStr string) ([]byte, error) {

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}
	return rule.uint16ToWire(uint16(port)), nil
}

// Family returns the family of the transport, 1 for IPv4 and 2 for IPv6.
func (rule *edns0VariableRule) family(ip net.Addr) int {
	var a net.IP
	if i, ok := ip.(*net.UDPAddr); ok {
		a = i.IP
	}
	if i, ok := ip.(*net.TCPAddr); ok {
		a = i.IP
	}
	if a.To4() != nil {
		return 1
	}
	return 2
}

// ruleData returns the data specified by the variable
func (rule *edns0VariableRule) ruleData(w dns.ResponseWriter, r *dns.Msg) ([]byte, error) {

	req := request.Request{W: w, Req: r}
	switch rule.variable {
	case queryName:
		//Query name is written as ascii string
		return []byte(req.QName()), nil

	case queryType:
		return rule.uint16ToWire(req.QType()), nil

	case clientIP:
		return rule.ipToWire(req.Family(), req.IP())

	case clientPort:
		return rule.portToWire(req.Port())

	case protocol:
		// Proto is written as ascii string
		return []byte(req.Proto()), nil

	case serverIP:
		ip, _, err := net.SplitHostPort(w.LocalAddr().String())
		if err != nil {
			ip = w.RemoteAddr().String()
		}
		return rule.ipToWire(rule.family(w.RemoteAddr()), ip)

	case serverPort:
		_, port, err := net.SplitHostPort(w.LocalAddr().String())
		if err != nil {
			port = "0"
		}
		return rule.portToWire(port)
	}

	return nil, fmt.Errorf("Unable to extract data for variable %s", rule.variable)
}

// Rewrite will alter the request EDNS0 local options with specified variables
func (rule *edns0VariableRule) Rewrite(w dns.ResponseWriter, r *dns.Msg) Result {
	result := RewriteIgnored

	data, err := rule.ruleData(w, r)
	if err != nil || data == nil {
		return result
	}

	o := setupEdns0Opt(r)
	found := false
	for _, s := range o.Option {
		switch e := s.(type) {
		case *dns.EDNS0_LOCAL:
			if rule.code == e.Code {
				if rule.action == Replace || rule.action == Set {
					e.Data = data
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
		opt.Data = data
		o.Option = append(o.Option, &opt)
		result = RewriteDone
	}

	return result
}

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
