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
)

func UpstreamSNI(u *structs.Upstream, subset string, dc string, trustDomain string) string {
	if u.Datacenter != "" {
		dc = u.Datacenter
	}

	if u.DestinationType == structs.UpstreamDestTypePreparedQuery {
		return QuerySNI(u.DestinationName, dc, trustDomain)
	}
	return ServiceSNI(u.DestinationName, subset, u.DestinationNamespace, u.DestinationPartition, dc, trustDomain)
}

func GatewaySNI(dc string, partition, trustDomain string) string {
	if partition == "" {
		// TODO(partitions) Make default available in OSS as a constant for uses like this one
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
		// TODO(partitions) Make default available in OSS as a constant for uses like this one
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

func dotJoin(parts ...string) string {
	return strings.Join(parts, ".")
}

func QuerySNI(service string, datacenter string, trustDomain string) string {
	return fmt.Sprintf("%s.default.%s.query.%s", service, datacenter, trustDomain)
}

func TargetSNI(target *structs.DiscoveryTarget, trustDomain string) string {
	return ServiceSNI(target.Service, target.ServiceSubset, target.Namespace, target.Partition, target.Datacenter, trustDomain)
}
