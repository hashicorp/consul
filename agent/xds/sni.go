package xds

import (
	"fmt"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func DatacenterSNI(dc string, cfgSnap *proxycfg.ConfigSnapshot) string {
	return fmt.Sprintf("%s.internal.%s", dc, cfgSnap.Roots.TrustDomain)
}

func ServiceSNI(service string, subset string, namespace string, datacenter string, cfgSnap *proxycfg.ConfigSnapshot) string {
	if subset == "" {
		return fmt.Sprintf("%s.%s.%s.internal.%s", service, namespace, datacenter, cfgSnap.Roots.TrustDomain)
	} else {
		return fmt.Sprintf("%s.%s.%s.%s.internal.%s", subset, service, namespace, datacenter, cfgSnap.Roots.TrustDomain)
	}
}

func TargetSNI(target structs.DiscoveryTarget, cfgSnap *proxycfg.ConfigSnapshot) string {
	return ServiceSNI(target.Service, target.ServiceSubset, target.Namespace, target.Datacenter, cfgSnap)
}
