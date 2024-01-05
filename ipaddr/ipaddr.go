// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ipaddr

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
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
