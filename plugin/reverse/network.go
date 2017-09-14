package reverse

import (
	"bytes"
	"net"
	"regexp"
	"strings"
)

type network struct {
	IPnet        *net.IPNet
	Zone         string // forward lookup zone
	Template     string
	TTL          uint32
	RegexMatchIP *regexp.Regexp
}

// TODO: we might want to get rid of these regexes.
const hexDigit = "0123456789abcdef"
const templateNameIP = "{ip}"
const regexMatchV4 = "((?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))"
const regexMatchV6 = "([0-9a-fA-F]{32})"

// hostnameToIP converts the hostname back to an ip, based on the template
// returns nil if there is no IP found.
func (network *network) hostnameToIP(rname string) net.IP {
	var matchedIP net.IP

	match := network.RegexMatchIP.FindStringSubmatch(rname)
	if len(match) != 2 {
		return nil
	}

	if network.IPnet.IP.To4() != nil {
		matchedIP = net.ParseIP(match[1])
	} else {
		// TODO: can probably just allocate a []byte and use that.
		var buf bytes.Buffer
		// convert back to an valid ipv6 string with colons
		for i := 0; i < 8*4; i += 4 {
			buf.WriteString(match[1][i : i+4])
			if i < 28 {
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

// ipToHostname converts an IP to an DNS compatible hostname and injects it into the template.domain.
func (network *network) ipToHostname(ip net.IP) (name string) {
	if ipv4 := ip.To4(); ipv4 != nil {
		// replace . to -
		name = ipv4.String()
	} else {
		// assume v6
		// ensure zeros are present in string
		buf := make([]byte, 0, len(ip)*4)
		for i := 0; i < len(ip); i++ {
			v := ip[i]
			buf = append(buf, hexDigit[v>>4])
			buf = append(buf, hexDigit[v&0xF])
		}
		name = string(buf)
	}
	// inject the converted ip into the fqdn template
	return strings.Replace(network.Template, templateNameIP, name, 1)
}

type networks []network

func (n networks) Len() int      { return len(n) }
func (n networks) Swap(i, j int) { n[i], n[j] = n[j], n[i] }

// cidr closer to the ip wins (by netmask)
func (n networks) Less(i, j int) bool {
	isize, _ := n[i].IPnet.Mask.Size()
	jsize, _ := n[j].IPnet.Mask.Size()
	return isize > jsize
}
