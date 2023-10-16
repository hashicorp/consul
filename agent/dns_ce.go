//go:build !consulent
// +build !consulent

package agent

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
)

type enterpriseDNSConfig struct{}

func getEnterpriseDNSConfig(conf *config.RuntimeConfig) enterpriseDNSConfig {
	return enterpriseDNSConfig{}
}

// parseLocality can parse peer name or datacenter from a DNS query's labels.
// Peer name is parsed from the same query part that datacenter is, so given this ambiguity
// we parse a "peerOrDatacenter". The caller or RPC handler are responsible for disambiguating.
func (d *DNSServer) parseLocality(labels []string, cfg *dnsConfig) (queryLocality, bool) {
	locality := queryLocality{
		EnterpriseMeta: d.defaultEnterpriseMeta,
	}

	switch len(labels) {
	case 2, 4:
		// Support the following formats:
		// - [.<datacenter>.dc]
		// - [.<peer>.peer]
		for i := 0; i < len(labels); i += 2 {
			switch labels[i+1] {
			case "dc":
				locality.datacenter = labels[i]
			case "peer":
				locality.peer = labels[i]
			default:
				return queryLocality{}, false
			}
		}
		// Return error when both datacenter and peer are specified.
		if locality.datacenter != "" && locality.peer != "" {
			return queryLocality{}, false
		}
		return locality, true
	case 1:
		return queryLocality{peerOrDatacenter: labels[0]}, true

	case 0:
		return queryLocality{}, true
	}

	return queryLocality{}, false
}

func serviceCanonicalDNSName(name, kind, datacenter, domain string, _ *acl.EnterpriseMeta) string {
	return fmt.Sprintf("%s.%s.%s.%s", name, kind, datacenter, domain)
}

func nodeCanonicalDNSName(lookup serviceLookup, nodeName, respDomain string) string {
	if lookup.PeerName != "" {
		// We must return a more-specific DNS name for peering so
		// that there is no ambiguity with lookups.
		return fmt.Sprintf("%s.node.%s.peer.%s",
			nodeName,
			lookup.PeerName,
			respDomain)
	}
	// Return a simpler format for non-peering nodes.
	return fmt.Sprintf("%s.node.%s.%s", nodeName, lookup.Datacenter, respDomain)
}
