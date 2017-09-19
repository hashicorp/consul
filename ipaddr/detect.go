package ipaddr

import (
	"fmt"
	"net"
)

// privateIPv4CIDRs contains the IPv4 address blocks which
// are used for private networks.
var privateIPv4CIDRs = []string{
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.168.0.0/16",
}

// privateIPv4Blocks contains the parsed privateCIDRs
var privateIPv4Blocks []*net.IPNet

func init() {
	for _, cidr := range privateIPv4CIDRs {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("Bad cidr %s. Got %v", cidr, err))
		}
		privateIPv4Blocks = append(privateIPv4Blocks, block)
	}
}

// GetPrivateIPv4 returns the list of private network IPv4 addresses on
// all active interfaces.
func GetPrivateIPv4() ([]net.IP, error) {
	addresses, err := activeInterfaceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Failed to get interface addresses: %v", err)
	}

	var addrs []net.IP
	for _, rawAddr := range addresses {
		var ip net.IP
		switch addr := rawAddr.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}
		if ip.To4() == nil {
			continue
		}
		if !isPrivateIPv4(ip.String()) {
			continue
		}
		addrs = append(addrs, ip)
	}
	return addrs, nil
}

// GetPublicIPv6 returns the list of all public IPv6 addresses
// on all active interfaces.
func GetPublicIPv6() ([]net.IP, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("Failed to get interface addresses: %v", err)
	}

	isUniqueLocalAddress := func(ip net.IP) bool {
		return len(ip) == net.IPv6len && ip[0] == 0xfc && ip[1] == 0x00
	}

	var addrs []net.IP
	for _, rawAddr := range addresses {
		var ip net.IP
		switch addr := rawAddr.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}
		if ip.To4() != nil {
			continue
		}
		if ip.IsLinkLocalUnicast() || isUniqueLocalAddress(ip) || ip.IsLoopback() {
			continue
		}
		addrs = append(addrs, ip)
	}
	return addrs, nil
}

// Returns if the given IP is in a private block
func isPrivateIPv4(ip_str string) bool {
	ip := net.ParseIP(ip_str)
	for _, priv := range privateIPv4Blocks {
		if priv.Contains(ip) {
			return true
		}
	}
	return false
}

// Returns addresses from interfaces that is up
func activeInterfaceAddresses() ([]net.Addr, error) {
	var upAddrs []net.Addr
	var loAddrs []net.Addr

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("Failed to get interfaces: %v", err)
	}

	for _, iface := range interfaces {
		// Require interface to be up
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addresses, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("Failed to get interface addresses: %v", err)
		}

		if iface.Flags&net.FlagLoopback != 0 {
			loAddrs = append(loAddrs, addresses...)
			continue
		}

		upAddrs = append(upAddrs, addresses...)
	}

	if len(upAddrs) == 0 {
		return loAddrs, nil
	}

	return upAddrs, nil
}
