package dnsutil

import "strings"

// ExtractAddressFromReverse turns a standard PTR reverse record name
// into an IP address. This works for ipv4 or ipv6.
//
// 54.119.58.176.in-addr.arpa. becomes 176.58.119.54. If the conversion
// failes the empty string is returned.
func ExtractAddressFromReverse(reverseName string) string {
	search := ""

	switch {
	case strings.HasSuffix(reverseName, v4arpaSuffix):
		search = strings.TrimSuffix(reverseName, v4arpaSuffix)
	case strings.HasSuffix(reverseName, v6arpaSuffix):
		search = strings.TrimSuffix(reverseName, v6arpaSuffix)
	default:
		return ""
	}

	// Reverse the segments and then combine them.
	segments := reverse(strings.Split(search, "."))
	return strings.Join(segments, ".")
}

func reverse(slice []string) []string {
	for i := 0; i < len(slice)/2; i++ {
		j := len(slice) - i - 1
		slice[i], slice[j] = slice[j], slice[i]
	}
	return slice
}

const (
	// v4arpaSuffix is the reverse tree suffix for v4 IP addresses.
	v4arpaSuffix = ".in-addr.arpa."
	// v6arpaSuffix is the reverse tree suffix for v6 IP addresses.
	v6arpaSuffix = ".ip6.arpa."
)
