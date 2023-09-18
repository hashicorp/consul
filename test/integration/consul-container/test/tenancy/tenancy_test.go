// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/internal/catalog"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

func TestTenancy(t *testing.T) {
	t.Parallel()

	// Create a single node cluster with the new resource APIs enabled.
	cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
			ACLEnabled: true,
		},
		Cmd: `-hcl=experiments=["resource-apis"]`,
	})
	require.Len(t, len(cluster.Servers()), 1)

	gvk := &api.GVK{
		Group:   catalog.ServiceV1Alpha1Type.Group,
		Version: catalog.ServiceV1Alpha1Type.GroupVersion,
		Kind:    catalog.ServiceV1Alpha1Type.Kind,
	}
	opts := &api.QueryOptions{
		Namespace:  "foo",
		Partition:  "bar",
		Datacenter: "dc1",
		Peer:       "local",
	}
	tenancy := &pbresource.Tenancy{
		Namespace: "foo",
		Partition: "bar",
		PeerName:  "local",
	}
	req := &api.WriteRequest{
		Owner: rtest.Resource(catalog.ServiceV1Alpha1Type, "api").WithTenancy(tenancy).ID(),
	}

	// Attempt to write a test resource in the "foo" namespace and "bar" partition.
	_, _, err := client.Resource().Apply(gvk, "test", opts, req)
	require.NoError(t, err)

	// Attempt to list resources in the "foo" namespace and "bar" partition.
	listRes, err := client.Resource().List(gvk, opts)
	require.NoError(t, err)
	require.Len(t, len(listRes.Resources), 1)

	// Attempt to delete the "api" resource in the "foo" namespace and "bar" partition.
	err = client.Resource().Delete(gvk, "api", opts)
	require.NoError(t, err)
}
