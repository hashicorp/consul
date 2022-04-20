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

func (d *DNSServer) parseDatacenterAndEnterpriseMeta(labels []string, _ *dnsConfig, datacenter *string, _ *acl.EnterpriseMeta) bool {
	switch len(labels) {
	case 1:
		*datacenter = labels[0]
		return true
	case 0:
		return true
	}
	return false
}

func serviceCanonicalDNSName(name, kind, datacenter, domain string, _ *acl.EnterpriseMeta) string {
	return fmt.Sprintf("%s.%s.%s.%s", name, kind, datacenter, domain)
}
