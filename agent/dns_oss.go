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
	switch len(labels) {
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
