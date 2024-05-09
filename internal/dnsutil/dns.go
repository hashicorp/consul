// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dnsutil

import (
	"net"
	"slices"
	"strings"

	"github.com/miekg/dns"
)

type TranslateAddressAccept int

const (
	arpaLabel     = "arpa"
	arpaIPV4Label = "in-addr"
	arpaIPV6Label = "ip6"
)

// IPFromARPA returns the net.IP address from a fully-qualified ARPA PTR domain name.
// If the address is an invalid format, it returns nil.
func IPFromARPA(arpa string) net.IP {
	labels := dns.SplitDomainName(arpa)
	if len(labels) != 6 && len(labels) != 34 {
		return nil
	}

	// The last two labels should be "in-addr" or "ip6" and "arpa"
	if labels[len(labels)-1] != arpaLabel {
		return nil
	}

	var ip net.IP
	switch labels[len(labels)-2] {
	case arpaIPV4Label:
		parts := labels[:len(labels)-2]
		slices.Reverse(parts)
		ip = net.ParseIP(strings.Join(parts, "."))
	case arpaIPV6Label:
		parts := labels[:len(labels)-2]
		slices.Reverse(parts)

		// Condense the different words of the address
		address := strings.Join(parts[0:4], "")
		for i := 4; i <= len(parts)-4; i = i + 4 {
			word := parts[i : i+4]
			address = address + ":" + strings.Join(word, "")
		}
		ip = net.ParseIP(address)
		// default: fallthrough
	}
	return ip
}
