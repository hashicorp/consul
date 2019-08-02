package xds

import (
	"fmt"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func UpstreamSNI(u *structs.Upstream, subset string, cfgSnap *proxycfg.ConfigSnapshot) string {
	if u.DestinationType == "prepared_query" {
		return QuerySNI(u.DestinationName, u.Datacenter, cfgSnap)
	}
	return ServiceSNI(u.DestinationName, subset, u.DestinationNamespace, u.Datacenter, cfgSnap)
}

func DatacenterSNI(dc string, cfgSnap *proxycfg.ConfigSnapshot) string {
	return fmt.Sprintf("%s.internal.%s", dc, cfgSnap.Roots.TrustDomain)
}

func ServiceSNI(service string, subset string, namespace string, datacenter string, cfgSnap *proxycfg.ConfigSnapshot) string {
	if namespace == "" {
		namespace = "default"
	}

	if datacenter == "" {
		datacenter = cfgSnap.Datacenter
	}

	if subset == "" {
		return fmt.Sprintf("%s.%s.%s.internal.%s", service, namespace, datacenter, cfgSnap.Roots.TrustDomain)
	} else {
		return fmt.Sprintf("%s.%s.%s.%s.internal.%s", subset, service, namespace, datacenter, cfgSnap.Roots.TrustDomain)
	}
}

func QuerySNI(service string, datacenter string, cfgSnap *proxycfg.ConfigSnapshot) string {
	if datacenter == "" {
		datacenter = cfgSnap.Datacenter
	}

	return fmt.Sprintf("%s.default.%s.query.%s", service, datacenter, cfgSnap.Roots.TrustDomain)
}

func TargetSNI(target *structs.DiscoveryTarget, cfgSnap *proxycfg.ConfigSnapshot) string {
	return ServiceSNI(target.Service, target.ServiceSubset, target.Namespace, target.Datacenter, cfgSnap)
}

func CustomizeClusterName(sni string, chain *structs.CompiledDiscoveryChain) string {
	if chain == nil || chain.CustomizationHash == "" {
		return sni
	}
	// Use a colon to delimit this prefix instead of a dot to avoid a
	// theoretical collision problem with subsets.
	return fmt.Sprintf("%s:%s", chain.CustomizationHash, sni)
}
