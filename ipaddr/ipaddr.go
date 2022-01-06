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

// In many cases we receive a network address with an optional port field.
// Well formed examples of this include:
// Description             Example
// <Hostname>              example.org
// <Hostname>:<Port>       example.org:1234
// <IPv4>                  120.0.0.1
// <IPv4>:<port>           127.0.0.1:1234
// <IPv6>                  ::1
// [<IPv6>]:<port>         [::1]:1234
// Mildly unusual but legal examples include:
// [<IPv4>]:<port>         [127.0.0.1]:1234
// Ill formed examples include:
// <IPv6>:port             ::1:1234

// Extend net.SplitHostPort to split address into host/ip and optional port fields. Returns -1 if no port. Attempts to deal with
// bare IPv6 address optimistically.
func SplitHostPort(address string) (host string, port int, err error) {
	var portString string
	host, portString, err = net.SplitHostPort(address)

	if err != nil {
		// IPv6 addresses have at least two colons. A single colon is treated as host:port or ip:port
		// We're assuming we either have a bare address w/o port
		byColon := strings.Split(address, ":")
		segments := len(byColon)
		switch segments {
		case 1:
			// Maybe we have a bare IPv4 or host name without a port
			if strings.Contains(err.Error(), "missing port in address") {
				host = byColon[0] // TODO validate this as IPv4 or hostname
				portString = ""
				err = nil
			}
			// no case for 2, because that should be picked up by the net.SplitHostPort above.
		case 3, 4, 5, 6, 7, 8:
			// Maybe we have a bare IPv6 address without a port (maybe check err for assistance?)
			if strings.HasPrefix(address, "[") && strings.HasSuffix(address, "]") {
				address = strings.TrimSuffix(strings.TrimPrefix(address, "["), "]")
			}

			if net.ParseIP(address) != nil {
				host = address
				portString = ""
				err = nil
			} else {

			}
		case 9: // special case for ill formed, but unambigous full ipv6+port
			host = strings.Join(byColon[:segments-1], ":")
			if net.ParseIP(host) != nil {
				portString = byColon[segments-1]
				err = nil
			} else {
				host = ""
				err = fmt.Errorf("Ill formed address %s. IPv6 addresses with port should be [IPv6]:port", address) // error?
			}
		default: // Note this includes the case were we have 2 segments
		}
	}

	if err != nil {
		host = ""
		port = -1
		return
	}
	if portString == "" {
		port = -1
		return
	}
	port, err = strconv.Atoi(portString)
	return
}

// Handle potentially malformed addresses with optional port fields.
func StripOptionalPort(address string) (host string, err error) {
	host, _, err = SplitHostPort(address)
	return
}
