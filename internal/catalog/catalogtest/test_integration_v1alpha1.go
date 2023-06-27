package catalogtest

import (
	"embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/endpoints"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

var (
	//go:embed integration_test_data
	testData embed.FS
)

// RunCatalogV1Alpha1IntegrationTest will push up a bunch of catalog related data and then
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
func RunCatalogV1Alpha1IntegrationTest(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	PublishCatalogV1Alpha1IntegrationTestData(t, client)
	VerifyCatalogV1Alpha1IntegrationTestResults(t, client)
}

// PublishCatalogV1Alpha1IntegrationTestData will perform a whole bunch of resource writes
// for Service, ServiceEndpoints, Workload, Node and HealthStatus objects
func PublishCatalogV1Alpha1IntegrationTestData(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	c := rtest.NewClient(client)

	resources := rtest.ParseResourcesFromFilesystem(t, testData, "integration_test_data/v1alpha1")
	c.PublishResources(t, resources)
}

func VerifyCatalogV1Alpha1IntegrationTestResults(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	c := rtest.NewClient(client)

	testutil.RunStep(t, "resources-exist", func(t *testing.T) {
		c.RequireResourceExists(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "api").ID())
		c.RequireResourceExists(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "http-api").ID())
		c.RequireResourceExists(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "grpc-api").ID())
		c.RequireResourceExists(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "foo").ID())

		for i := 1; i < 5; i++ {
			nodeId := rtest.Resource(catalog.NodeV1Alpha1Type, fmt.Sprintf("node-%d", i)).ID()
			c.RequireResourceExists(t, nodeId)

			res := c.RequireResourceExists(t, rtest.Resource(catalog.HealthStatusV1Alpha1Type, fmt.Sprintf("node-%d-health", i)).ID())
			rtest.RequireOwner(t, res, nodeId, true)
		}

		for i := 1; i < 21; i++ {
			workloadId := rtest.Resource(catalog.WorkloadV1Alpha1Type, fmt.Sprintf("api-%d", i)).ID()
			c.RequireResourceExists(t, workloadId)

			res := c.RequireResourceExists(t, rtest.Resource(catalog.HealthStatusV1Alpha1Type, fmt.Sprintf("api-%d-health", i)).ID())
			rtest.RequireOwner(t, res, workloadId, true)
		}
	})

	testutil.RunStep(t, "node-health-reconciliation", func(t *testing.T) {
		c.WaitForStatusCondition(t, rtest.Resource(catalog.NodeV1Alpha1Type, "node-1").ID(), nodehealth.StatusKey, nodehealth.ConditionPassing)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.NodeV1Alpha1Type, "node-2").ID(), nodehealth.StatusKey, nodehealth.ConditionWarning)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.NodeV1Alpha1Type, "node-3").ID(), nodehealth.StatusKey, nodehealth.ConditionCritical)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.NodeV1Alpha1Type, "node-4").ID(), nodehealth.StatusKey, nodehealth.ConditionMaintenance)
	})

	testutil.RunStep(t, "workload-health-reconciliation", func(t *testing.T) {
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-1").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadPassing)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-2").ID(), workloadhealth.StatusKey, workloadhealth.ConditionWorkloadWarning)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-3").ID(), workloadhealth.StatusKey, workloadhealth.ConditionWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-4").ID(), workloadhealth.StatusKey, workloadhealth.ConditionWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-5").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeWarning)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-6").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadWarning)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-7").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-8").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-9").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeCritical)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-10").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-11").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-12").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-13").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-14").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-15").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-16").ID(), workloadhealth.StatusKey, workloadhealth.ConditionNodeAndWorkloadMaintenance)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-17").ID(), workloadhealth.StatusKey, workloadhealth.ConditionWorkloadPassing)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-18").ID(), workloadhealth.StatusKey, workloadhealth.ConditionWorkloadWarning)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-19").ID(), workloadhealth.StatusKey, workloadhealth.ConditionWorkloadCritical)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-20").ID(), workloadhealth.StatusKey, workloadhealth.ConditionWorkloadMaintenance)
	})

	testutil.RunStep(t, "service-reconciliation", func(t *testing.T) {
		c.WaitForStatusCondition(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "foo").ID(), endpoints.StatusKey, endpoints.ConditionUnmanaged)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "api").ID(), endpoints.StatusKey, endpoints.ConditionManaged)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "http-api").ID(), endpoints.StatusKey, endpoints.ConditionManaged)
		c.WaitForStatusCondition(t, rtest.Resource(catalog.ServiceV1Alpha1Type, "grpc-api").ID(), endpoints.StatusKey, endpoints.ConditionManaged)
	})

	testutil.RunStep(t, "service-endpoints-generation", func(t *testing.T) {
		verifyServiceEndpoints(t, c, rtest.Resource(catalog.ServiceEndpointsV1Alpha1Type, "foo").ID(), expectedFooServiceEndpoints())
		verifyServiceEndpoints(t, c, rtest.Resource(catalog.ServiceEndpointsV1Alpha1Type, "api").ID(), expectedApiServiceEndpoints(t, c))
		verifyServiceEndpoints(t, c, rtest.Resource(catalog.ServiceEndpointsV1Alpha1Type, "http-api").ID(), expectedHTTPApiServiceEndpoints(t, c))
		verifyServiceEndpoints(t, c, rtest.Resource(catalog.ServiceEndpointsV1Alpha1Type, "grpc-api").ID(), expectedGRPCApiServiceEndpoints(t, c))
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
					"external-service-port": {
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
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-1").ID()),
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
			},
			// api-2
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-2").ID()),
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
			},
			// api-3
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-3").ID()),
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
			},
			// api-4
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-4").ID()),
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
			},
			// api-5
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-5").ID()),
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
			},
			// api-6
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-6").ID()),
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
			},
			// api-7
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-7").ID()),
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
			},
			// api-8
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-8").ID()),
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
			},
			// api-9
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-9").ID()),
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
			},
			// api-10
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-10").ID()),
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
			},
			// api-11
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-11").ID()),
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
			},
			// api-12
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-12").ID()),
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
			},
			// api-13
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-13").ID()),
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
			},
			// api-14
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-14").ID()),
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
			},
			// api-15
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-15").ID()),
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
			},
			// api-16
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-16").ID()),
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
			},
			// api-17
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-17").ID()),
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
			},
			// api-18
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-18").ID()),
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
			},
			// api-19
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-19").ID()),
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
			},
			// api-20
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-20").ID()),
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
			},
		},
	}
}

func expectedHTTPApiServiceEndpoints(t *testing.T, c *rtest.Client) *pbcatalog.ServiceEndpoints {
	return &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			// api-1
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-1").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.1", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
			// api-10
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-10").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.10", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
			// api-11
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-11").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.11", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
			// api-12
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-12").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.12", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
			},
			// api-13
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-13").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.13", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
			},
			// api-14
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-14").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.14", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
			},
			// api-15
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-15").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.15", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
			},
			// api-16
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-16").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.16", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
			},
			// api-17
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-17").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.17", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
			// api-18
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-18").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.18", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
			},
			// api-19
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-19").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.19", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
		},
	}
}

func expectedGRPCApiServiceEndpoints(t *testing.T, c *rtest.Client) *pbcatalog.ServiceEndpoints {
	return &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			// api-1
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-1").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.1", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.1", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
			// api-2
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-2").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.2", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.2", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
			},
			// api-3
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-3").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.3", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.3", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
			// api-4
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-4").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.4", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.4", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
			},
			// api-5
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-5").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.5", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.5", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
			},
			// api-6
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-6").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.6", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.6", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_WARNING,
			},
			// api-7
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-7").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.7", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.7", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
			// api-8
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-8").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.8", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.8", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
			},
			// api-9
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-9").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.9", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.9", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
			// api-20
			{
				TargetRef: c.ResolveResourceID(t, rtest.Resource(types.WorkloadV1Alpha1Type, "api-20").ID()),
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "172.16.1.20", Ports: []string{"grpc", "mesh"}},
					{Host: "198.18.2.20", External: true, Ports: []string{"mesh"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				HealthStatus: pbcatalog.Health_HEALTH_MAINTENANCE,
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
