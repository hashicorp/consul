// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authtest

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/require"

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

// RunAuthV1Alpha1IntegrationTest will push up a bunch of auth related data and then
// verify that all the expected reconciliations happened correctly.
// Besides just controller reconciliation behavior, the intent is also to verify
// that integrations with the resource service are also working (i.e. the various
// validation, mutation and ACL hooks get invoked and are working properly)
func RunAuthV1Alpha1IntegrationTest(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	PublishAuthV1Alpha1IntegrationTestData(t, client)
	VerifyAuthV1Alpha1IntegrationTestResults(t, client)
}

// PublishAuthV1Alpha1IntegrationTestData will perform a whole bunch of resource writes
// for WorkloadIdentity, TrafficPermission, NamespaceTrafficPermisison, and PartitionTrafficPermission objects
func PublishAuthV1Alpha1IntegrationTestData(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	c := rtest.NewClient(client)
	resources := rtest.ParseResourcesFromFilesystem(t, testData, "integration_test_data/v1alpha1/validation")
	for _, res := range resources {
		PublishInputWithValidationTest(t, c, res)
	}
}

func PublishInputWithValidationTest(t *testing.T, client pbresource.ResourceServiceClient, res *pbresource.Resource) {
	resourceErrs := map[string]string{
		"tp-1": "",
		"tp-2": "",
		"tp-3": "invalid \"data.action\" field: action must be either allow or deny",
		"tp-4": "invalid element at index 0 of list \"data.destination\": prefix values must be valid and not combined with explicit names",
		"tp-5": "invalid \"data.destination\" field: prefix values must be valid and not combined with explicit names",
		"tp-6": "invalid element at index 0 of list \"sources\": permissions must contain at least one source",
		"tp-7": "invalid \"data.destination\" field: cannot be empty",
		"tp-8": "permissions sources may not specify partitions, peers, and sameness_groups together",
	}

	_, err := client.Write(testutil.TestContext(t), &pbresource.WriteRequest{
		Resource: res,
	})
	if res.Id.Name != "tp-1" && res.Id.Name != "tp-2" {
		require.ErrorContains(t, err, resourceErrs[res.Id.Name])
	} else {
		require.NoError(t, err)
	}
}

func VerifyAuthV1Alpha1IntegrationTestResults(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	c := rtest.NewClient(client)

	testutil.RunStep(t, "resources-exist", func(t *testing.T) {
		c.RequireResourceExists(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-1").ID())
		c.RequireResourceExists(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-2").ID())
		c.RequireResourceNotFound(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-3").ID())
		c.RequireResourceNotFound(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-4").ID())
		c.RequireResourceNotFound(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-5").ID())
		c.RequireResourceNotFound(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-6").ID())
		c.RequireResourceNotFound(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-7").ID())
		c.RequireResourceNotFound(t, rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-8").ID())

		tp1 := rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-1").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, tp1)
		tp2 := rtest.Resource(auth.TrafficPermissionsV1Alpha1Type, "tp-2").WithTenancy(resource.DefaultNamespacedTenancy()).ID()
		c.RequireResourceExists(t, tp2)
	})
}
