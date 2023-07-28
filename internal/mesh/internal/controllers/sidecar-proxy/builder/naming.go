package builder

import (
	"fmt"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func DestinationClusterName(serviceRef *pbresource.Reference, datacenter, trustDomain string) string {
	return connect.ServiceSNI(serviceRef.Name,
		"",
		serviceRef.Tenancy.Namespace,
		serviceRef.Tenancy.Partition,
		datacenter,
		trustDomain)
}

func DestinationStatPrefix(serviceRef *pbresource.Reference, datacenter string) string {
	return fmt.Sprintf("upstream.%s.%s.%s.%s",
		serviceRef.Name,
		serviceRef.Tenancy.Namespace,
		serviceRef.Tenancy.Partition,
		datacenter)
}

func DestinationListenerName(name, portName string, address string, port uint32) string {
	if port != 0 {
		return fmt.Sprintf("%s:%s:%s:%d", name, portName, address, port)
	}

	return fmt.Sprintf("%s:%s:%s", name, portName, address)
}
