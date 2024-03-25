// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ipaddr

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
)

// FormatAddressPort Helper for net.JoinHostPort that takes int for port
func FormatAddressPort(address string, port int) string {
	return net.JoinHostPort(address, strconv.Itoa(port))
}

// IsAny checks if the given ip address is an IPv4 or IPv6 ANY address. ip
// can be either a *net.IP or a string. It panics on another type.
func IsAny(ip interface{}) bool {
	return IsAnyV4(ip) || IsAnyV6(ip)
}

// IsAnyV4 checks if the given ip address is an IPv4 ANY address. ip
// can be either a *net.IP or a string. It panics on another type.
func IsAnyV4(ip interface{}) bool {
	return iptos(ip) == "0.0.0.0"
}

// IsAnyV6 checks if the given ip address is an IPv6 ANY address. ip
// can be either a *net.IP or a string. It panics on another type.
func IsAnyV6(ip interface{}) bool {
	ips := iptos(ip)
	return ips == "::" || ips == "[::]"
}

func iptos(ip interface{}) string {
	if ip == nil || reflect.TypeOf(ip).Kind() == reflect.Ptr && reflect.ValueOf(ip).IsNil() {
		return ""
	}
	switch x := ip.(type) {
	case string:
		return x
	case *string:
		if x == nil {
			return ""
		}
		return *x
	case net.IP:
		return x.String()
	case *net.IP:
		return x.String()
	case *net.IPAddr:
		return x.IP.String()
	case *net.TCPAddr:
		return x.IP.String()
	case *net.UDPAddr:
		return x.IP.String()
	default:
		panic(fmt.Sprintf("invalid type: %T", ip))
	}
}

// The HasPort and EnsurePort functions are copied from unexported versions in
// https://github.com/hashicorp/memberlist/blob/master/util.go.
// They are needed when wanting to parse Consul's retry_join configuration for auto_encrypt, with the same semantics
// as the underlying memberlist library, and may also be useful for other cases of processing user input where a port
// number is optional, as Go's standard library does not provide good support for that.

// HasPort is given a string of the form "host", "host:port", "ipv6::address",
// or "[ipv6::address]:port", and returns true if the string includes a port.
func HasPort(s string) bool {
	// IPv6 address in brackets.
	if strings.LastIndex(s, "[") == 0 {
		return strings.LastIndex(s, ":") > strings.LastIndex(s, "]")
	}

	// Otherwise the presence of a single colon determines if there's a port
	// since IPv6 addresses outside of brackets (count > 1) can't have a
	// port.
	return strings.Count(s, ":") == 1
}

// EnsurePort makes sure the given string has a port number on it, otherwise it
// appends the given port as a default.
func EnsurePort(s string, port int) string {
	if HasPort(s) {
		return s
	}

	// If this is an IPv6 address, the join call will add another set of
	// brackets, so we have to trim before we add the default port.
	s = strings.Trim(s, "[]")
	s = net.JoinHostPort(s, strconv.Itoa(port))
	return s
}
