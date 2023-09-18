package connect

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	internal        = "internal"
	version         = "v1"
	internalVersion = internal + "-" + version
	external        = "external"
)

func UpstreamSNI(u *structs.Upstream, subset string, dc string, trustDomain string) string {
	if u.Datacenter != "" {
		dc = u.Datacenter
	}

	if u.DestinationType == structs.UpstreamDestTypePreparedQuery {
		return QuerySNI(u.DestinationName, dc, trustDomain)
	}
	// TODO(peering): account for peer here?
	return ServiceSNI(u.DestinationName, subset, u.DestinationNamespace, u.DestinationPartition, dc, trustDomain)
}

func GatewaySNI(dc string, partition, trustDomain string) string {
	if partition == "" {
		// TODO(partitions) Make default available in CE as a constant for uses like this one
		partition = "default"
	}

	switch partition {
	case "default":
		return dotJoin(dc, internal, trustDomain)
	default:
		return dotJoin(partition, dc, internalVersion, trustDomain)
	}
}

func ServiceSNI(service string, subset string, namespace string, partition string, datacenter string, trustDomain string) string {
	if namespace == "" {
		namespace = structs.IntentionDefaultNamespace
	}
	if partition == "" {
		// TODO(partitions) Make default available in CE as a constant for uses like this one
		partition = "default"
	}

	switch partition {
	case "default":
		if subset == "" {
			return dotJoin(service, namespace, datacenter, internal, trustDomain)
		} else {
			return dotJoin(subset, service, namespace, datacenter, internal, trustDomain)
		}
	default:
		if subset == "" {
			return dotJoin(service, namespace, partition, datacenter, internalVersion, trustDomain)
		} else {
			return dotJoin(subset, service, namespace, partition, datacenter, internalVersion, trustDomain)
		}
	}
}

func dotSplitLast(s string, n int) string {
	tokens := strings.SplitN(s, ".", n)
	if len(tokens) != n {
		return ""
	}
	return tokens[n-1]
}

func TrustDomainForTarget(target structs.DiscoveryTarget) string {
	if target.External {
		return ""
	}

	switch target.Partition {
	case "default":
		if target.ServiceSubset == "" {
			// service, namespace, datacenter, internal, trustDomain
			return dotSplitLast(target.SNI, 5)
		} else {
			// subset, service, namespace, datacenter, internal, trustDomain
			return dotSplitLast(target.SNI, 6)
		}
	default:
		if target.ServiceSubset == "" {
			// service, namespace, partition, datacenter, internalVersion, trustDomain
			return dotSplitLast(target.SNI, 6)
		} else {
			// subset, service, namespace, partition, datacenter, internalVersion, trustDomain
			return dotSplitLast(target.SNI, 7)
		}
	}
}

func PeeredServiceSNI(service, namespace, partition, peerName, trustDomain string) string {
	if peerName == "" {
		panic("peer name is a requirement for this function and does not make sense without it")
	}
	if namespace == "" {
		namespace = structs.IntentionDefaultNamespace
	}
	if partition == "" {
		// TODO(partitions) Make default available in CE as a constant for uses like this one
		partition = "default"
	}

	return dotJoin(service, namespace, partition, peerName, external, trustDomain)
}

func dotJoin(parts ...string) string {
	return strings.Join(parts, ".")
}

func QuerySNI(service string, datacenter string, trustDomain string) string {
	return fmt.Sprintf("%s.default.%s.query.%s", service, datacenter, trustDomain)
}

func TargetSNI(target *structs.DiscoveryTarget, trustDomain string) string {
	return ServiceSNI(target.Service, target.ServiceSubset, target.Namespace, target.Partition, target.Datacenter, trustDomain)
}
