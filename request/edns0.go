package request

import (
	"github.com/coredns/coredns/plugin/pkg/edns"

	"github.com/miekg/dns"
)

func supportedOptions(o []dns.EDNS0) []dns.EDNS0 {
	var supported = make([]dns.EDNS0, 0, 3)
	// For as long as possible try avoid looking up in the map, because that need an Rlock.
	for _, opt := range o {
		switch code := opt.Option(); code {
		case dns.EDNS0NSID:
			fallthrough
		case dns.EDNS0EXPIRE:
			fallthrough
		case dns.EDNS0COOKIE:
			fallthrough
		case dns.EDNS0TCPKEEPALIVE:
			fallthrough
		case dns.EDNS0PADDING:
			supported = append(supported, opt)
		default:
			if edns.SupportedOption(code) {
				supported = append(supported, opt)
			}
		}
	}
	return supported
}
