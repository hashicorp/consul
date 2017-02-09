package reverse

import (
	"net"
	"regexp"
	"bytes"
	"strings"
)

type network struct {
	IPnet        *net.IPNet
	Zone 	     string // forward lookup zone
	Template     string
	TTL          uint32
	RegexMatchIP *regexp.Regexp
	Fallthrough  bool
}

const hexDigit = "0123456789abcdef"
const templateNameIP = "{ip}"
const regexMatchV4 = "((?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\-){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))"
const regexMatchV6 = "([0-9a-fA-F]{32})"

// For forward lookup
// converts the hostname back to an ip, based on the template
// returns nil if there is no ip found
func (network *network) hostnameToIP(rname string) net.IP {
	var matchedIP net.IP

	// use precompiled regex by setup
	match := network.RegexMatchIP.FindStringSubmatch(rname)
	// regex did not matched
	if (len(match) != 2) {
		return nil
	}

	if network.IPnet.IP.To4() != nil {
		matchedIP = net.ParseIP(strings.Replace(match[1], "-", ".", 4))
	} else {
		var buf bytes.Buffer
		// convert back to an valid ipv6 string with colons
		for i := 0; i < 8 * 4; i += 4 {
			buf.WriteString(match[1][i:i + 4])
			if (i < 28) {
				buf.WriteString(":")
			}
		}
		matchedIP = net.ParseIP(buf.String())
	}

	// No valid ip or it does not belong to this network
	if matchedIP == nil || !network.IPnet.Contains(matchedIP) {
		return nil
	}

	return matchedIP
}

// For reverse lookup
// Converts an Ip to an dns compatible hostname and injects it into the template.domain
func (network *network) ipToHostname(ip net.IP) string {
	var name string

	ipv4 := ip.To4()
	if ipv4 != nil {
		// replace . to -
		name = uitoa(ipv4[0]) + "-" +
			uitoa(ipv4[1]) + "-" +
			uitoa(ipv4[2]) + "-" +
			uitoa(ipv4[3])
	} else {
		// assume v6
		// ensure zeros are present in string
		buf := make([]byte, 0, len(ip) * 4)
		for i := 0; i < len(ip); i++ {
			v := ip[i]
			buf = append(buf, hexDigit[v >> 4])
			buf = append(buf, hexDigit[v & 0xF])
		}
		name = string(buf)
	}
	// inject the converted ip into the fqdn template
	return strings.Replace(network.Template, templateNameIP, name, 1)
}

// just the same from net.ip package, but with uint8
func uitoa(val uint8) string {
	if val == 0 {
		// avoid string allocation
		return "0"
	}
	var buf [20]byte // big enough for 64bit value base 10
	i := len(buf) - 1
	for val >= 10 {
		q := val / 10
		buf[i] = byte('0' + val - q * 10)
		i--
		val = q
	}
	// val < 10
	buf[i] = byte('0' + val)
	return string(buf[i:])
}

type networks []network

// implements the sort interface
func (slice networks) Len() int {
	return len(slice)
}

// implements the sort interface
// cidr closer to the ip wins (by netmask)
func (slice networks) Less(i, j int) bool {
	isize, _ := slice[i].IPnet.Mask.Size()
	jsize, _ := slice[j].IPnet.Mask.Size()
	return isize > jsize
}

// implements the sort interface
func (slice networks) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}