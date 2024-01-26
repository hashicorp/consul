// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogtest

import (
	"embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/endpoints"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

var (
	//go:embed integration_test_data
	testData embed.FS
)

// RunCatalogV2Beta1IntegrationTest will push up a bunch of catalog related data and then
// verify that all the expected reconciliations happened correctly. This test is
// intended to exercise a large swathe of behavior of the overall catalog package.
// Besides just controller reconciliation behavior, the intent is also to verify
// that integrations with the resource service are also working (i.e. the various
// validation, mutation and ACL hooks get invoked and are working properly)
//
// This test specifically is not doing any sort of lifecycle related tests to ensure
// that modification to values results in re-reconciliation as expected. Instead there
// is another RunCatalogIntegrationTestLifeCycle function that can be used for those
// purposes. The two are distinct so that the data being published and the assertions
// made against the system can be reused in upgrade tests.
func RunCatalogV2Beta1IntegrationTest(t *testing.T, client pbresource.ResourceServiceClient, opts ...rtest.ClientOption) {
	t.Helper()

	PublishCatalogV2Beta1IntegrationTestData(t, client, opts...)
	VerifyCatalogV2Beta1IntegrationTestResults(t, client)
}

// PublishCatalogV2Beta1IntegrationTestData will perform a whole bunch of resource writes
// for Service, ServiceEndpoints, Workload, Node and HealthStatus objects
func PublishCatalogV2Beta1IntegrationTestData(t *testing.T, client pbresource.ResourceServiceClient, opts ...rtest.ClientOption) {
	t.Helper()

	c := rtest.NewClient(client, opts...)

	resources := rtest.ParseResourcesFromFilesystem(t, testData, "integration_test_data/v2beta1")
	c.PublishResources(t, resources)
}

func VerifyCatalogV2Beta1IntegrationTestResults(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	c := rtest.NewClient(client)

	testutil.RunStep(t, "resources-exist", func(t *testing.T) {
		// When this test suite is run against multiple servers with the resource service client
		// pointed at a Raft follower, there can be a race between Raft replicating all the data
		// to the followers and these verifications running. Instead of wrapping each one of these
		// in their own Wait/Retry func the whole set of them is being wrapped.
		retry.Run(t, func(r *retry.R) {
			c.RequireResourceExists(r, rtest.Resource(pbcatalog.ServiceType, "api").ID())
			c.RequireResourceExists(r, rtest.Resource(pbcatalog.ServiceType, "http-api").ID())
			c.RequireResourceExists(r, rtest.Resource(pbcatalog.ServiceType, "grpc-api").ID())
			c.RequireResourceExists(r, rtest.Resource(pbcatalog.ServiceType, "foo").ID())

			for i := 1; i < 5; i++ {
				nodeId := rtest.Resource(pbcatalog.NodeType, fmt.Sprintf("node-%d", i)).WithTenancy(resource.DefaultPartitionedTenancy()).ID()
				c.RequireResourceExists(r, nodeId)

				res := c.RequireResourceExists(r, rtest.Resource(pbcatalog.NodeHealthStatusType, fmt.Sprintf("node-%d-health", i)).ID())
				rtest.RequireOwner(r, res, nodeId, true)
			}

			for i := 1; i < 21; i++ {
				workloadId := rtest.Resource(pbcatalog.WorkloadType, fmt.Sprintf("api-%d", i)).WithTenancy(resource.DefaultNamespacedTenancy()).ID()
				c.RequireResourceExists(r, workloadId)

				res := c.RequireResourceExists(r, rtest.Resource(pbcatalog.HealthStatusType, fmt.Sprintf("api-%d-health", i)).ID())
				rtest.RequireOwner(r, res, workloadId, true)
			}
		},
			// Using a 2 second retry because Raft replication really ought to be this fast for our integration
			// tests and if the test hardware cannot get 100 logs replicated to all followers in 2 seconds or
			// less then we have some serious issues that warrant investigation.
			retry.WithRetryer(retry.TwoSeconds()),
		)
	})

	testutil.RunStep(t, "node-health-reconciliation", func(t *testing.T) {
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.NodeType, "node-1").WithTenancy(resource.DefaultPartitionedTenancy()).ID(), nodehealth.StatusKey, nodehealth.ConditionPassing)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.NodeType, "node-2").WithTenancy(resource.DefaultPartitionedTenancy()).ID(), nodehealth.StatusKey, nodehealth.ConditionWarning)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.NodeType, "node-3").WithTenancy(resource.DefaultPartitionedTenancy()).ID(), nodehealth.StatusKey, nodehealth.ConditionCritical)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.NodeType, "node-4").WithTenancy(resource.DefaultPartitionedTenancy()).ID(), nodehealth.StatusKey, nodehealth.ConditionMaintenance)
	})

	testutil.RunStep(t, "workload-health-reconciliation", func(t *testing.T) {
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-1").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadPassing)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-2").ID(), workloadhealth.ControllerID, workloadhealth.ConditionWorkloadWarning)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-3").ID(), workloadhealth.ControllerID, workloadhealth.ConditionWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-4").ID(), workloadhealth.ControllerID, workloadhealth.ConditionWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-5").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeWarning)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-6").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadWarning)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-7").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-8").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-9").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeCritical)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-10").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-11").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-12").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-13").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-14").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-15").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-16").ID(), workloadhealth.ControllerID, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-17").ID(), workloadhealth.ControllerID, workloadhealth.ConditionWorkloadPassing)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-18").ID(), workloadhealth.ControllerID, workloadhealth.ConditionWorkloadWarning)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-19").ID(), workloadhealth.ControllerID, workloadhealth.ConditionWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.WorkloadType, "api-20").ID(), workloadhealth.ControllerID, workloadhealth.ConditionWorkloadMaintenance)
	})

	testutil.RunStep(t, "service-reconciliation", func(t *testing.T) {
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.ServiceType, "foo").ID(), endpoints.ControllerID, endpoints.ConditionUnmanaged)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.ServiceType, "api").ID(), endpoints.ControllerID, endpoints.ConditionManaged)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.ServiceType, "http-api").ID(), endpoints.ControllerID, endpoints.ConditionManaged)
		c.WaitForStatusCondition(t, rtest.Resource(pbcatalog.ServiceType, "grpc-api").ID(), endpoints.ControllerID, endpoints.ConditionManaged)
	})

	testutil.RunStep(t, "service-endpoints-generation", func(t *testing.T) {
		verifyServiceEndpoints(t, c, rtest.Resource(pbcatalog.ServiceEndpointsType, "foo").ID(), expectedFooServiceEndpoints())
		verifyServiceEndpoints(t, c, rtest.Resource(pbcatalog.ServiceEndpointsType, "api").ID(), expectedApiServiceEndpoints(t, c))
		verifyServiceEndpoints(t, c, rtest.Resource(pbcatalog.ServiceEndpointsType, "http-api").ID(), expectedHTTPApiServiceEndpoints(t, c))
		verifyServiceEndpoints(t, c, rtest.Resource(pbcatalog.ServiceEndpointsType, "grpc-api").ID(), expectedGRPCApiServiceEndpoints(t, c))
	})
}

func expectedFooServiceEndpoints() *pbcatalog.ServiceEndpoints {
	return &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "198.18.0.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"ext-svc-port": {
						Port:     9876,
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2,
					},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
		},
	}
}

func expectedApiServiceEndpoints(t *testing.T, c *rtest.Client) *pbcatalog.ServiceEndpoints {
	return &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			// api-1
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-1").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.1", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.1", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
				Identity:     "api",
			},
			// api-2
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-2").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.2", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.2", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-3
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-3").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.3", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.3", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-4
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-4").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.4", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.4", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-5
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-5").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.5", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.5", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-6
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-6").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.6", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.6", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-7
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-7").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.7", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.7", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-8
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-8").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.8", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.8", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-9
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-9").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.9", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.9", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-10
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-10").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.10", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.10", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-11
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-11").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.11", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.11", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-12
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-12").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.12", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.12", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-13
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-13").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.13", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.13", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-14
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-14").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.14", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.14", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-15
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-15").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.15", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.15", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-16
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-16").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.16", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.16", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-17
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-17").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.17", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.17", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
				Identity:     "api",
			},
			// api-18
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-18").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.18", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.18", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-19
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-19").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.19", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.19", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-20
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-20").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.20", Ports: []string{"grpc", "http", "mesh"}},
					{Host: "198.18.2.20", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
		},
	}
}

func expectedHTTPApiServiceEndpoints(t *testing.T, c *rtest.Client) *pbcatalog.ServiceEndpoints {
	return &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			// api-1
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-1").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.1", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
				Identity:     "api",
			},
			// api-10
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-10").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.10", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-11
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-11").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.11", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-12
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-12").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.12", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-13
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-13").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.13", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-14
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-14").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.14", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-15
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-15").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.15", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-16
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-16").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.16", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-17
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-17").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.17", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
				Identity:     "api",
			},
			// api-18
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-18").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.18", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-19
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-19").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.19", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
		},
	}
}

func expectedGRPCApiServiceEndpoints(t *testing.T, c *rtest.Client) *pbcatalog.ServiceEndpoints {
	return &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			// api-1
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-1").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.1", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.1", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
				Identity:     "api",
			},
			// api-2
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-2").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.2", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.2", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-3
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-3").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.3", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.3", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-4
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-4").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.4", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.4", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-5
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-5").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.5", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.5", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-6
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-6").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.6", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.6", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
				Identity:     "api",
			},
			// api-7
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-7").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.7", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.7", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-8
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-8").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.8", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.8", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
			// api-9
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-9").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.9", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.9", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				Identity:     "api",
			},
			// api-20
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(pbcatalog.WorkloadType, "api-20").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.20", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.20", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
				Identity:     "api",
			},
		},
	}
}

func verifyServiceEndpoints(t *testing.T, c *rtest.Client, id *pbresource.ID, expected *pbcatalog.ServiceEndpoints) {
	t.Helper()
	c.WaitForResourceState(t, id, func(t rtest.T, res *pbresource.Resource) {
		var actual pbcatalog.ServiceEndpoints
		err := res.Data.UnmarshalTo(&actual)
		require.NoError(t, err)
		prototest.AssertElementsMatch(t, expected.Endpoints, actual.Endpoints)
	})
}
