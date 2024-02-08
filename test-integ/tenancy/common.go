// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancy

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
	"github.com/hashicorp/consul/test-integ/topoutil"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

const (
	DefaultNamespaceName = "default"
	DefaultPartitionName = "default"
)

func newConfig(t *testing.T) *topology.Config {
	const clusterName = "cluster1"
	servers := topoutil.NewTopologyServerSet(clusterName+"-server", 3, []string{clusterName}, nil)

	cluster := &topology.Cluster{
		Enterprise:      utils.IsEnterprise(),
		Name:            clusterName,
		Nodes:           servers,
		EnableV2:        true,
		EnableV2Tenancy: true,
	}

	return &topology.Config{
		Images:   utils.TargetImages(),
		Networks: []*topology.Network{{Name: clusterName}},
		Clusters: []*topology.Cluster{cluster},
	}
}

func createNamespaces(t *testing.T, resourceServiceClient *Client, numNamespaces int, ap string) []*pbresource.Resource {
	namespaces := []*pbresource.Resource{}
	for i := 0; i < numNamespaces; i++ {
		namespace := &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    fmt.Sprintf("namespace-%d", i),
				Type:    pbtenancy.NamespaceType,
				Tenancy: &pbresource.Tenancy{Partition: ap},
			},
		}
		rsp, err := resourceServiceClient.Write(context.Background(), &pbresource.WriteRequest{Resource: namespace})
		require.NoError(t, err)
		namespace = resourceServiceClient.WaitForResourceExists(t, rsp.Resource.Id)
		namespaces = append(namespaces, namespace)
	}
	return namespaces
}

func createServices(t *testing.T, resourceServiceClient *Client, numServices int, ap string, ns string) []*pbresource.Resource {
	services := []*pbresource.Resource{}
	for i := 0; i < numServices; i++ {
		service := &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    fmt.Sprintf("service-%d", i),
				Type:    pbcatalog.ServiceType,
				Tenancy: &pbresource.Tenancy{Partition: ap, Namespace: ns},
			},
		}
		service = sprawltest.MustSetResourceData(t, service, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{},
			Ports:     []*pbcatalog.ServicePort{},
		})
		rsp, err := resourceServiceClient.Write(context.Background(), &pbresource.WriteRequest{Resource: service})
		require.NoError(t, err)
		service = resourceServiceClient.WaitForResourceExists(t, rsp.Resource.Id)
		services = append(services, service)
	}
	return services
}
