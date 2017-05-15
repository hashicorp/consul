package ipaddr

import (
	"fmt"
	"net"
)

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
	if ip == nil {
		return ""
	}
	switch x := ip.(type) {
	case string:
		return x
	case net.IP:
		return x.String()
	case *net.IP:
		return x.String()
	default:
		panic(fmt.Sprintf("invalid type: %T", ip))
	}
}
