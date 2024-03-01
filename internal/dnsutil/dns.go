// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dnsutil

import (
	"errors"
	"net"
	"regexp"
	"slices"
	"strings"

	"github.com/miekg/dns"
)

type TranslateAddressAccept int

// MaxLabelLength is the maximum length for a name that can be used in DNS.
const (
	MaxLabelLength = 63

	arpaLabel     = "arpa"
	arpaIPV4Label = "in-addr"
	arpaIPV6Label = "ip6"

	TranslateAddressAcceptDomain TranslateAddressAccept = 1 << iota
	TranslateAddressAcceptIPv4
	TranslateAddressAcceptIPv6

	TranslateAddressAcceptAny TranslateAddressAccept = ^0
)

// InvalidNameRe is a regex that matches characters which can not be included in
// a DNS name.
var InvalidNameRe = regexp.MustCompile(`[^A-Za-z0-9\\-]+`)

// matches valid DNS labels according to RFC 1123, should be at most 63
// characters according to the RFC
var validLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

// IsValidLabel returns true if the string given is a valid DNS label (RFC 1123).
// Note: the only difference between RFC 1035 and RFC 1123 labels is that in
// RFC 1123 labels can begin with a number.
func IsValidLabel(name string) bool {
	return validLabel.MatchString(name)
}

// ValidateLabel is similar to IsValidLabel except it returns an error
// instead of false when name is not a valid DNS label. The error will contain
// reference to what constitutes a valid DNS label.
func ValidateLabel(name string) error {
	if !IsValidLabel(name) {
		return errors.New("a valid DNS label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}
	return nil
}

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
