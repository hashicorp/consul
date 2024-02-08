// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package tenancy

import (
	"testing"

	"github.com/stretchr/testify/require"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
)

// TestNamespaceLifecycle sets up the following:
//
// - 1 cluster
// - 3 servers in that cluster
// - v2 resources and v2 tenancy are activated
//
// When this test is executed it tests the full lifecycle for a
// small number of namespaces:
// - creation of namespaces in the default partition
// - populating resources under namespaces
// - finally deleting everything
func TestNamespaceLifecycle(t *testing.T) {
	t.Parallel()

	cfg := newConfig(t)
	sp := sprawltest.Launch(t, cfg)
	cluster := sp.Topology().Clusters["cluster1"]
	client := NewClient(sp.ResourceServiceClientForCluster(cluster.Name))

	// 3 namespaces
	// @ 3 services per namespace
	// ==============================
	// 9 resources total
	tenants := []*pbresource.Resource{}
	numNamespaces := 3
	numServices := 3

	// Default namespace is expected to exist
	// when we boostrap a cluster
	client.RequireResourceExists(t, &pbresource.ID{
		Name:    DefaultNamespaceName,
		Type:    pbtenancy.NamespaceType,
		Tenancy: &pbresource.Tenancy{Partition: DefaultPartitionName},
	})

	// Namespaces are created in default partition
	namespaces := createNamespaces(t, client, numNamespaces, DefaultPartitionName)

	for _, namespace := range namespaces {
		services := createServices(t, client, numServices, DefaultPartitionName, namespace.Id.Name)
		tenants = append(tenants, services...)
	}

	// Verify test setup
	require.Equal(t, len(tenants), numNamespaces*numServices)

	// List namespaces
	listRsp, err := client.List(client.Context(t), &pbresource.ListRequest{
		Type:       pbtenancy.NamespaceType,
		Tenancy:    &pbresource.Tenancy{},
		NamePrefix: "namespace-",
	})
	require.NoError(t, err)
	require.Equal(t, len(namespaces), len(listRsp.Resources))

	// Delete all namespaces
	for _, namespace := range namespaces {
		_, err := client.Delete(client.Context(t), &pbresource.DeleteRequest{Id: namespace.Id})
		require.NoError(t, err)
		client.WaitForDeletion(t, namespace.Id)
	}

	// Make sure no namespace tenants left behind
	for _, tenant := range tenants {
		client.RequireResourceNotFound(t, tenant.Id)
	}
}
