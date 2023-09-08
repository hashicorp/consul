// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authtest

import (
	"embed"
	"testing"

	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

var (
	//go:embed integration_test_data
	testData embed.FS
)

// RunCatalogV1Alpha1IntegrationTest will push up a bunch of auth related data and then
// verify that all the expected reconciliations happened correctly.
// Besides just controller reconciliation behavior, the intent is also to verify
// that integrations with the resource service are also working (i.e. the various
// validation, mutation and ACL hooks get invoked and are working properly)
func RunCatalogV1Alpha1IntegrationTest(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	PublishCatalogV1Alpha1IntegrationTestData(t, client)
	VerifyCatalogV1Alpha1IntegrationTestResults(t, client)
}

// PublishCatalogV1Alpha1IntegrationTestData will perform a whole bunch of resource writes
// for WorkloadIdentity, TrafficPermission, NamespaceTrafficPermisison, and PartitionTrafficPermission objects
func PublishCatalogV1Alpha1IntegrationTestData(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	c := rtest.NewClient(client)

	resources := rtest.ParseResourcesFromFilesystem(t, testData, "integration_test_data/v1alpha1/upgrade")
	c.PublishResources(t, resources)
}

func VerifyCatalogV1Alpha1IntegrationTestResults(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	c := rtest.NewClient(client)

	testutil.RunStep(t, "resources-exist", func(t *testing.T) {
		c.RequireResourceExists(t, rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-1").ID())
		c.RequireResourceExists(t, rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-2").ID())
		c.RequireResourceExists(t, rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-3").ID())
		//c.RequireResourceExists(t, rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-4").ID())
		c.RequireResourceExists(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-1").ID())
		c.RequireResourceExists(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-2").ID())
		c.RequireResourceExists(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-3").ID())
		//c.RequireResourceExists(t, rtest.Resource(auth.NamespaceTrafficPermissionsV1Alpha1Type, "ns-tp-1").ID())
		//c.RequireResourceExists(t, rtest.Resource(auth.PartitionTrafficPermissionsV1Alpha1Type, "pn-tp-1").ID())

		wi1 := rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-1").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, wi1)
		wi2 := rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-2").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, wi2)
		wi3 := rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-2").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, wi3)
		//wi3 := rtest.Resource(auth.WorkloadIdentityV1Alpha1Type, "wi-4").WithTenancy(&pbresource.Tenancy{
		//	Partition: resource.DefaultPartitionName,
		//	Namespace: "ns1",
		//}).ID()
		//c.RequireResourceExists(t, wi4)

		tp1 := rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-1").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, tp1)
		tp2 := rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-1").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, tp2)
		tp3 := rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-1").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, tp3)
	})
}
