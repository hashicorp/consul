package connect

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

func UpstreamSNI(u *structs.Upstream, subset string, dc string, trustDomain string) string {
	if u.Datacenter != "" {
		dc = u.Datacenter
	}

	if u.DestinationType == structs.UpstreamDestTypePreparedQuery {
		return QuerySNI(u.DestinationName, dc, trustDomain)
	}
	return ServiceSNI(u.DestinationName, subset, u.DestinationNamespace, dc, trustDomain)
}

func DatacenterSNI(dc string, trustDomain string) string {
	return fmt.Sprintf("%s.internal.%s", dc, trustDomain)
}

func ServiceSNI(service string, subset string, namespace string, datacenter string, trustDomain string) string {
	if namespace == "" {
		namespace = "default"
	}

	if subset == "" {
		return fmt.Sprintf("%s.%s.%s.internal.%s", service, namespace, datacenter, trustDomain)
	} else {
		return fmt.Sprintf("%s.%s.%s.%s.internal.%s", subset, service, namespace, datacenter, trustDomain)
	}
}

func QuerySNI(service string, datacenter string, trustDomain string) string {
	return fmt.Sprintf("%s.default.%s.query.%s", service, datacenter, trustDomain)
}

func TargetSNI(target *structs.DiscoveryTarget, trustDomain string) string {
	return ServiceSNI(target.Service, target.ServiceSubset, target.Namespace, target.Datacenter, trustDomain)
}
