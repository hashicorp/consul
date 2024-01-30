// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package dns

import (
	"github.com/miekg/dns"
	"net"
	"strings"
)

func newDNSAddress(addr string) *dnsAddress {
	a := &dnsAddress{}
	a.SetAddress(addr)
	return a
}

// dnsAddress is a wrapper around a string that represents a DNS address and
// provides helper methods for determining whether it is an IP or FQDN and
// whether it is internal or external to the domain.
type dnsAddress struct {
	addr string

	// store an IP so helpers don't have to parse it multiple times
	ip net.IP
}

// SetAddress sets the address field and the ip field if the string is an IP.
func (a *dnsAddress) SetAddress(addr string) {
	a.addr = addr
	a.ip = net.ParseIP(addr)
}

// IP returns the IP address if the address is an IP.
func (a *dnsAddress) IP() net.IP {
	return a.ip
}

// IsIP returns true if the address is an IP.
func (a *dnsAddress) IsIP() bool {
	return a.IP() != nil
}

// IsIPV4 returns true if the address is an IPv4 address.
func (a *dnsAddress) IsIPV4() bool {
	if a.IP() == nil {
		return false
	}
	return a.IP().To4() != nil
}

// FQDN returns the FQDN if the address is not an IP.
func (a *dnsAddress) FQDN() string {
	if !a.IsEmptyString() && !a.IsIP() {
		return dns.Fqdn(a.addr)
	}
	return ""
}

// IsFQDN returns true if the address is a FQDN and not an IP.
func (a *dnsAddress) IsFQDN() bool {
	return !a.IsEmptyString() && !a.IsIP() && dns.IsFqdn(a.FQDN())
}

// String returns the address as a string.
func (a *dnsAddress) String() string {
	return a.addr
}

// IsEmptyString returns true if the address is an empty string.
func (a *dnsAddress) IsEmptyString() bool {
	return a.addr == ""
}

// IsInternalFQDN returns true if the address is a FQDN and is internal to the domain.
func (a *dnsAddress) IsInternalFQDN(domain string) bool {
	return !a.IsIP() && a.IsFQDN() && strings.HasSuffix(a.FQDN(), domain)
}

// IsInternalFQDNOrIP returns true if the address is an IP or a FQDN and is internal to the domain.
func (a *dnsAddress) IsInternalFQDNOrIP(domain string) bool {
	return a.IsIP() || a.IsInternalFQDN(domain)
}

// IsExternalFQDN returns true if the address is a FQDN and is external to the domain.
func (a *dnsAddress) IsExternalFQDN(domain string) bool {
	return !a.IsIP() && a.IsFQDN() && strings.Count(a.FQDN(), ".") > 1 && !strings.HasSuffix(a.FQDN(), domain)
}
