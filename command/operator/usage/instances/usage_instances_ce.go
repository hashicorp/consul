//go:build !consulent
// +build !consulent

package instances

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
)

const showPartitionNamespace = false

func getBillableInstanceCounts(usage api.ServiceUsage, datacenter string) []serviceCount {
	return []serviceCount{
		{
			datacenter:    datacenter,
			partition:     acl.DefaultPartitionName,
			namespace:     acl.DefaultNamespaceName,
			instanceCount: usage.BillableServiceInstances,
			services:      usage.Services,
		},
	}
}

func getConnectInstanceCounts(usage api.ServiceUsage, datacenter string) []serviceCount {
	var counts []serviceCount

	for serviceType, instanceCount := range usage.ConnectServiceInstances {
		counts = append(counts, serviceCount{
			datacenter:    datacenter,
			partition:     acl.DefaultPartitionName,
			namespace:     acl.DefaultNamespaceName,
			serviceType:   serviceType,
			instanceCount: instanceCount,
		})
	}

	return counts
}
