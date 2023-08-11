// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogtest

import (
	"testing"

	"github.com/hashicorp/consul/internal/catalog"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

// RunCatalogV1Alpha1LifecycleIntegrationTest intends to excercise functionality of
// managing catalog resources over their normal lifecycle where they will be modified
// several times, change state etc.
func RunCatalogV1Alpha1LifecycleIntegrationTest(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()

	testutil.RunStep(t, "node-lifecycle", func(t *testing.T) {
		RunCatalogV1Alpha1NodeLifecycleIntegrationTest(t, client)
	})

	testutil.RunStep(t, "workload-lifecycle", func(t *testing.T) {
		RunCatalogV1Alpha1WorkloadLifecycleIntegrationTest(t, client)
	})

	testutil.RunStep(t, "endpoints-lifecycle", func(t *testing.T) {
		RunCatalogV1Alpha1EndpointsLifecycleIntegrationTest(t, client)
	})
}

// RunCatalogV1Alpha1NodeLifecycleIntegrationTest verifies correct functionality of
// the node-health controller. This test will exercise the following behaviors:
//
// * Creating a Node without associated HealthStatuses will mark the node as passing
// * Associating a HealthStatus with a Node will cause recomputation of the Health
// * Changing HealthStatus to a worse health will cause recomputation of the Health
// * Changing HealthStatus to a better health will cause recomputation of the Health
// * Deletion of associated HealthStatuses will recompute the Health (back to passing)
// * Deletion of the node will cause deletion of associated health statuses
func RunCatalogV1Alpha1NodeLifecycleIntegrationTest(t *testing.T, client pbresource.ResourceServiceClient) {
	c := rtest.NewClient(client)

	nodeName := "test-lifecycle"
	nodeHealthName := "test-lifecycle-node-status"

	// initial node creation
	node := rtest.Resource(catalog.NodeV1Alpha1Type, nodeName).
		WithData(t, &pbcatalog.Node{
			Addresses: []*pbcatalog.NodeAddress{
				{Host: "172.16.2.3"},
				{Host: "198.18.2.3", External: true},
			},
		}).
		Write(t, c)

	// wait for the node health controller to mark the node as healthy
	c.WaitForStatusCondition(t, node.Id,
		catalog.NodeHealthStatusKey,
		catalog.NodeHealthConditions[pbcatalog.Health_HEALTH_PASSING])

	// Its easy enough to simply repeatedly set the health status and it proves
	// that going both from better to worse health and worse to better all
	// happen as expected. We leave the health in a warning state to allow for
	// the subsequent health status deletion to cause the health to go back
	// to passing.
	healthChanges := []pbcatalog.Health{
		pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
	}

	// This will be set within the loop and used afterwards to delete the health status
	var nodeHealth *pbresource.Resource

	// Iterate through the various desired health statuses, updating
	// a HealthStatus resource owned by the node and waiting for
	// reconciliation at each point
	for _, health := range healthChanges {
		// update the health check
		nodeHealth = setHealthStatus(t, c, node.Id, nodeHealthName, health)

		// wait for reconciliation to kick in and put the node into the right
		// health status.
		c.WaitForStatusCondition(t, node.Id,
			catalog.NodeHealthStatusKey,
			catalog.NodeHealthConditions[health])
	}

	// now delete the health status and ensure things go back to passing
	c.MustDelete(t, nodeHealth.Id)

	// wait for the node health controller to mark the node as healthy
	c.WaitForStatusCondition(t, node.Id,
		catalog.NodeHealthStatusKey,
		catalog.NodeHealthConditions[pbcatalog.Health_HEALTH_PASSING])

	// Add the health status back once more, the actual status doesn't matter.
	// It just must be owned by the node so that we can show cascading
	// deletions of owned health statuses working.
	healthStatus := setHealthStatus(t, c, node.Id, nodeHealthName, pbcatalog.Health_HEALTH_CRITICAL)

	// Delete the node and wait for the health status to be deleted.
	c.MustDelete(t, node.Id)
	c.WaitForDeletion(t, healthStatus.Id)
}

// RunCatalogV1Alpha1WorkloadLifecycleIntegrationTest verifies correct functionality of
// the workload-health controller. This test will exercise the following behaviors:
//
//   - Associating a workload with a node causes recomputation of the health and takes
//     into account the nodes health
//   - Modifying the workloads associated node causes health recomputation and takes into
//     account the new nodes health
//   - Removal of the node association causes recomputation of health and for no node health
//     to be taken into account.
//   - Creating a workload without associated health statuses or node association will
//     be marked passing
//   - Creating a workload without associated health statuses but with a node will
//     inherit its health from the node.
//   - Changing HealthStatus to a worse health will cause recompuation of the Health
//   - Changing HealthStatus to a better health will cause recompuation of the Health
//   - Overall health is computed as the worst health amongst the nodes health and all
//     of the workloads associated HealthStatuses
//   - Deletion of the workload will cause deletion of all associated health statuses.
func RunCatalogV1Alpha1WorkloadLifecycleIntegrationTest(t *testing.T, client pbresource.ResourceServiceClient) {
	c := rtest.NewClient(client)
	testutil.RunStep(t, "nodeless-workload", func(t *testing.T) {
		runV1Alpha1NodelessWorkloadLifecycleIntegrationTest(t, c)
	})

	testutil.RunStep(t, "node-associated-workload", func(t *testing.T) {
		runV1Alpha1NodeAssociatedWorkloadLifecycleIntegrationTest(t, c)
	})
}

// runV1Alpha1NodelessWorkloadLifecycleIntegrationTest verifies correct functionality of
// the workload-health controller for workloads without node associations. In particular
// the following behaviors are being tested
//
//   - Creating a workload without associated health statuses or node association will
//     be marked passing
//   - Changing HealthStatus to a worse health will cause recompuation of the Health
//   - Changing HealthStatus to a better health will cause recompuation of the Health
//   - Deletion of associated HealthStatus for a nodeless workload will be set back to passing
//   - Deletion of the workload will cause deletion of all associated health statuses.
func runV1Alpha1NodelessWorkloadLifecycleIntegrationTest(t *testing.T, c *rtest.Client) {
	workloadName := "test-lifecycle-workload"
	workloadHealthName := "test-lifecycle-workload-status"

	// create a workload without a node association or health statuses yet
	workload := rtest.Resource(catalog.WorkloadV1Alpha1Type, workloadName).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "198.18.9.8"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "test-lifecycle",
		}).
		Write(t, c)

	// wait for the workload health controller to mark the workload as healthy
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadHealthConditions[pbcatalog.Health_HEALTH_PASSING])

	// We may not need to iterate through all of these states but its easy
	// enough and quick enough to do so. The general rationale is that we
	// should move through changing the workloads associated health status
	// in this progression. We can prove that moving from better to worse
	// health or worse to better both function correctly.
	healthChanges := []pbcatalog.Health{
		pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
	}

	var workloadHealth *pbresource.Resource
	// Iterate through the various desired health statuses, updating
	// a HealthStatus resource owned by the workload and waiting for
	// reconciliation at each point
	for _, health := range healthChanges {
		// update the health status
		workloadHealth = setHealthStatus(t, c, workload.Id, workloadHealthName, health)

		// wait for reconciliation to kick in and put the workload into
		// the right health status.
		c.WaitForStatusCondition(t, workload.Id,
			catalog.WorkloadHealthStatusKey,
			catalog.WorkloadHealthConditions[health])
	}

	// Now delete the health status, things should go back to passing status
	c.MustDelete(t, workloadHealth.Id)

	// ensure the workloads health went back to passing
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadHealthConditions[pbcatalog.Health_HEALTH_PASSING])

	// Reset the workload health. The actual health is irrelevant, we just want it
	// to exist to provde that Health Statuses get deleted along with the workload
	// when its deleted.
	workloadHealth = setHealthStatus(t, c, workload.Id, workloadHealthName, pbcatalog.Health_HEALTH_WARNING)

	// Delete the workload and wait for the HealthStatus to also be deleted
	c.MustDelete(t, workload.Id)
	c.WaitForDeletion(t, workloadHealth.Id)
}

// runV1Alpha1NodeAssociatedWorkloadLifecycleIntegrationTest verifies correct functionality of
// the workload-health controller. This test will exercise the following behaviors:
//
//   - Associating a workload with a node causes recomputation of the health and takes
//     into account the nodes health
//   - Modifying the workloads associated node causes health recomputation and takes into
//     account the new nodes health
//   - Removal of the node association causes recomputation of health and for no node health
//     to be taken into account.
//   - Creating a workload without associated health statuses but with a node will
//     inherit its health from the node.
//   - Overall health is computed as the worst health amongst the nodes health and all
//     of the workloads associated HealthStatuses
func runV1Alpha1NodeAssociatedWorkloadLifecycleIntegrationTest(t *testing.T, c *rtest.Client) {
	workloadName := "test-lifecycle"
	workloadHealthName := "test-lifecycle"
	nodeName1 := "test-lifecycle-1"
	nodeName2 := "test-lifecycle-2"
	nodeHealthName1 := "test-lifecycle-node-1"
	nodeHealthName2 := "test-lifecycle-node-2"

	// Insert a some nodes to link the workloads to at various points throughout the test
	node1 := rtest.Resource(catalog.NodeV1Alpha1Type, nodeName1).
		WithData(t, &pbcatalog.Node{
			Addresses: []*pbcatalog.NodeAddress{{Host: "172.17.9.10"}},
		}).
		Write(t, c)
	node2 := rtest.Resource(catalog.NodeV1Alpha1Type, nodeName2).
		WithData(t, &pbcatalog.Node{
			Addresses: []*pbcatalog.NodeAddress{{Host: "172.17.9.11"}},
		}).
		Write(t, c)

	// Set some non-passing health statuses for those nodes. Using non-passing will make
	// it easy to see that changing a passing workloads node association appropriately
	// impacts the overall workload health.
	setHealthStatus(t, c, node1.Id, nodeHealthName1, pbcatalog.Health_HEALTH_CRITICAL)
	setHealthStatus(t, c, node2.Id, nodeHealthName2, pbcatalog.Health_HEALTH_WARNING)

	// Add the workload but don't immediately associate with any node.
	workload := rtest.Resource(catalog.WorkloadV1Alpha1Type, workloadName).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "198.18.9.8"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "test-lifecycle",
		}).
		Write(t, c)

	// wait for the workload health controller to mark the workload as healthy
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadHealthConditions[pbcatalog.Health_HEALTH_PASSING])

	// now modify the workload to associate it with node 1 (currently with CRITICAL health)
	workload = rtest.ResourceID(workload.Id).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "198.18.9.8"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
			Identity:  "test-lifecycle",
			// this is the only difference from the previous write
			NodeName: node1.Id.Name,
		}).
		Write(t, c)

	// wait for the workload health controller to mark the workload as critical (due to node 1 having critical health)
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadAndNodeHealthConditions[pbcatalog.Health_HEALTH_PASSING][pbcatalog.Health_HEALTH_CRITICAL])

	// Now reassociate the workload with node 2. This should cause recalculation of its health into the warning state
	workload = rtest.ResourceID(workload.Id).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "198.18.9.8"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
			Identity:  "test-lifecycle",
			// this is the only difference from the previous write
			NodeName: node2.Id.Name,
		}).
		Write(t, c)

	// Wait for the workload health controller to mark the workload as warning (due to node 2 having warning health)
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadAndNodeHealthConditions[pbcatalog.Health_HEALTH_PASSING][pbcatalog.Health_HEALTH_WARNING])

	// Delete the node, this should cause the health to be recalculated as critical because the node association
	// is broken.
	c.MustDelete(t, node2.Id)

	// Wait for the workload health controller to mark the workload as critical due to the missing node
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadAndNodeHealthConditions[pbcatalog.Health_HEALTH_PASSING][pbcatalog.Health_HEALTH_CRITICAL])

	// Now fixup the node association to point at node 1
	workload = rtest.ResourceID(workload.Id).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "198.18.9.8"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
			Identity:  "test-lifecycle",
			// this is the only difference from the previous write
			NodeName: node1.Id.Name,
		}).
		Write(t, c)

	// Also set node 1 health down to WARNING
	setHealthStatus(t, c, node1.Id, nodeHealthName1, pbcatalog.Health_HEALTH_WARNING)

	// Wait for the workload health controller to mark the workload as warning (due to node 1 having warning health now)
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadAndNodeHealthConditions[pbcatalog.Health_HEALTH_PASSING][pbcatalog.Health_HEALTH_WARNING])

	// Now add a critical workload health check to ensure that both node and workload health are accounted for.
	setHealthStatus(t, c, workload.Id, workloadHealthName, pbcatalog.Health_HEALTH_CRITICAL)

	// Wait for the workload health to be recomputed and put into the critical status.
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadAndNodeHealthConditions[pbcatalog.Health_HEALTH_CRITICAL][pbcatalog.Health_HEALTH_WARNING])

	// Reset the workloads health to passing. We expect the overall health to go back to warning
	setHealthStatus(t, c, workload.Id, workloadHealthName, pbcatalog.Health_HEALTH_PASSING)
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadAndNodeHealthConditions[pbcatalog.Health_HEALTH_PASSING][pbcatalog.Health_HEALTH_WARNING])

	// Remove the node association and wait for the health to go back to passing
	workload = rtest.ResourceID(workload.Id).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "198.18.9.8"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
			Identity:  "test-lifecycle",
		}).
		Write(t, c)
	c.WaitForStatusCondition(t, workload.Id,
		catalog.WorkloadHealthStatusKey,
		catalog.WorkloadHealthConditions[pbcatalog.Health_HEALTH_PASSING])
}

// RunCatalogV1Alpha1EndpointsLifecycleIntegrationTest verifies the correct functionality of
// the endpoints controller. This test will exercise the following behaviors:
//
// * Services without a selector get marked with status indicating their endpoints are unmanaged
// * Services with a selector get marked with status indicating their endpoints are managed
// * Deleting a service will delete the associated endpoints (regardless of them being managed or not)
// * Moving from managed to unmanaged endpoints will delete the managed endpoints
// * Moving from unmanaged to managed endpoints will overwrite any previous endpoints.
// * A service with a selector that matches no workloads will still have the endpoints object written.
// * Adding ports to a service will recalculate the endpoints
// * Removing ports from a service will recalculate the endpoints
// * Changing the workload will recalculate the endpoints (ports, addresses, or health)
func RunCatalogV1Alpha1EndpointsLifecycleIntegrationTest(t *testing.T, client pbresource.ResourceServiceClient) {
	c := rtest.NewClient(client)
	serviceName := "test-lifecycle"

	// Create the service without a selector. We should not see endpoints generated but we should see the
	// status updated to note endpoints are not being managed.
	service := rtest.Resource(catalog.ServiceV1Alpha1Type, serviceName).
		WithData(t, &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
		}).
		Write(t, c)

	// Wait to ensure the status is updated accordingly
	c.WaitForStatusCondition(t, service.Id, catalog.EndpointsStatusKey, catalog.EndpointsStatusConditionUnmanaged)

	// Verify that no endpoints were created.
	endpointsID := rtest.Resource(catalog.ServiceEndpointsV1Alpha1Type, serviceName).ID()
	c.RequireResourceNotFound(t, endpointsID)

	// Add some empty endpoints (type validations enforce that they are owned by the service)
	rtest.ResourceID(endpointsID).
		WithData(t, &pbcatalog.ServiceEndpoints{}).
		WithOwner(service.Id).
		Write(t, c)

	// Now delete the service and ensure that they are cleaned up.
	c.MustDelete(t, service.Id)
	c.WaitForDeletion(t, endpointsID)

	// Add some workloads to eventually select by the service

	// api-1 has all ports (http, grpc and mesh). It also has a mixture of Addresses
	// that select individual ports and one that selects all ports implicitly
	api1 := rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-1").
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
				{Host: "::1", Ports: []string{"grpc"}},
				{Host: "127.0.0.2", Ports: []string{"http"}},
				{Host: "172.17.1.1", Ports: []string{"mesh"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			},
			Identity: "api",
		}).
		Write(t, c)

	// api-2 has only grpc and mesh ports. It also has a mixture of Addresses that
	// select individual ports and one that selects all ports implicitly
	api2 := rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-2").
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
				{Host: "::1", Ports: []string{"grpc"}},
				{Host: "172.17.1.2", Ports: []string{"mesh"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			},
			Identity: "api",
		}).
		Write(t, c)

	// api-3 has the mesh and HTTP ports. It also has a mixture of Addresses that
	// select individual ports and one that selects all ports.
	api3 := rtest.Resource(catalog.WorkloadV1Alpha1Type, "api-3").
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
				{Host: "172.17.1.3", Ports: []string{"mesh"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "api",
		}).
		Write(t, c)

	// Now create a service with unmanaged endpoints again
	service = rtest.Resource(catalog.ServiceV1Alpha1Type, serviceName).
		WithData(t, &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
		}).
		Write(t, c)

	// Inject the endpoints resource. We want to prove that transition from unmanaged to
	// managed endpoints results in overwriting of the old endpoints
	rtest.ResourceID(endpointsID).
		WithData(t, &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					Addresses: []*pbcatalog.WorkloadAddress{
						{Host: "198.18.1.1", External: true},
					},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"http": {Port: 443, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					},
					HealthStatus: pbcatalog.Health_HEALTH_PASSING,
				},
			},
		}).
		WithOwner(service.Id).
		Write(t, c)

	// Wait to ensure the status is updated accordingly
	c.WaitForStatusCondition(t, service.Id, catalog.EndpointsStatusKey, catalog.EndpointsStatusConditionUnmanaged)

	// Now move the service to having managed endpoints
	service = rtest.ResourceID(service.Id).
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Names: []string{"bar"}},
			Ports:     []*pbcatalog.ServicePort{{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
		}).
		Write(t, c)

	// Verify that this status is updated to show this service as having managed endpoints
	c.WaitForStatusCondition(t, service.Id, catalog.EndpointsStatusKey, catalog.EndpointsStatusConditionManaged)

	// Verify that the service endpoints are created. In this case they will be empty
	verifyServiceEndpoints(t, c, endpointsID, &pbcatalog.ServiceEndpoints{})

	// Rewrite the service to select the API workloads - just select the singular port for now
	service = rtest.ResourceID(service.Id).
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
			Ports:     []*pbcatalog.ServicePort{{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP}},
		}).
		Write(t, c)

	// Wait for the status to be updated. The condition itself will remain unchanged but we are waiting for
	// the generations to match to know that the endpoints would have been regenerated
	c.WaitForStatusCondition(t, service.Id, catalog.EndpointsStatusKey, catalog.EndpointsStatusConditionManaged)

	// ensure that api-1 and api-3 are selected but api-2 is excluded due to not having the desired port
	verifyServiceEndpoints(t, c, endpointsID, &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: api1.Id,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "127.0.0.1", Ports: []string{"http"}},
					{Host: "127.0.0.2", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
			{
				TargetRef: api3.Id,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "127.0.0.1", Ports: []string{"http"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
		},
	})

	// Rewrite the service to select the API workloads - changing from selecting the HTTP port to the gRPC port
	service = rtest.ResourceID(service.Id).
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
			Ports:     []*pbcatalog.ServicePort{{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC}},
		}).
		Write(t, c)

	// Wait for the status to be updated. The condition itself will remain unchanged but we are waiting for
	// the generations to match to know that the endpoints would have been regenerated
	c.WaitForStatusCondition(t, service.Id, catalog.EndpointsStatusKey, catalog.EndpointsStatusConditionManaged)

	// Check that the endpoints were generated as expected
	verifyServiceEndpoints(t, c, endpointsID, &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: api1.Id,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "127.0.0.1", Ports: []string{"grpc"}},
					{Host: "::1", Ports: []string{"grpc"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
			{
				TargetRef: api2.Id,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "127.0.0.1", Ports: []string{"grpc"}},
					{Host: "::1", Ports: []string{"grpc"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
		},
	})

	// Update the service to change the ports used. This should result in the workload being removed
	// from the endpoints
	rtest.ResourceID(api2.Id).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
				{Host: "::1", Ports: []string{"http"}},
				{Host: "172.17.1.2", Ports: []string{"mesh"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Identity: "api",
		}).
		Write(t, c)

	// Verify that api-2 was removed from the service endpoints as it no longer has a grpc port
	verifyServiceEndpoints(t, c, endpointsID, &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: api1.Id,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "127.0.0.1", Ports: []string{"grpc"}},
					{Host: "::1", Ports: []string{"grpc"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
		},
	})

	// Remove the ::1 address from workload api1 which should result in recomputing endpoints
	rtest.ResourceID(api1.Id).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
				{Host: "172.17.1.1", Ports: []string{"mesh"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"mesh": {Port: 10000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			},
			Identity: "api",
		}).
		Write(t, c)

	// Verify that api-1 had its addresses modified appropriately
	verifyServiceEndpoints(t, c, endpointsID, &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: api1.Id,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "127.0.0.1", Ports: []string{"grpc"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
		},
	})

	// Add a failing health status to the api1 workload to force recomputation of endpoints
	setHealthStatus(t, c, api1.Id, "api-failed", pbcatalog.Health_HEALTH_CRITICAL)

	// Verify that api-1 within the endpoints has the expected health
	verifyServiceEndpoints(t, c, endpointsID, &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: api1.Id,
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "127.0.0.1", Ports: []string{"grpc"}},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
		},
	})

	// Move the service to being unmanaged. We should see the ServiceEndpoints being removed.
	service = rtest.ResourceID(service.Id).
		WithData(t, &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC}},
		}).
		Write(t, c)

	// Wait for the endpoints controller to inform us that the endpoints are not being managed
	c.WaitForStatusCondition(t, service.Id, catalog.EndpointsStatusKey, catalog.EndpointsStatusConditionUnmanaged)
	// Ensure that the managed endpoints were deleted
	c.WaitForDeletion(t, endpointsID)

	// Put the service back into managed mode.
	service = rtest.ResourceID(service.Id).
		WithData(t, &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
			Ports:     []*pbcatalog.ServicePort{{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC}},
		}).
		Write(t, c)

	// Wait for the service endpoints to be regenerated
	c.WaitForStatusCondition(t, service.Id, catalog.EndpointsStatusKey, catalog.EndpointsStatusConditionManaged)
	c.RequireResourceExists(t, endpointsID)

	// Now delete the service and ensure that the endpoints eventually are deleted as well
	c.MustDelete(t, service.Id)
	c.WaitForDeletion(t, endpointsID)

}

func setHealthStatus(t *testing.T, client *rtest.Client, owner *pbresource.ID, name string, health pbcatalog.Health) *pbresource.Resource {
	return rtest.Resource(catalog.HealthStatusV1Alpha1Type, name).
		WithData(t, &pbcatalog.HealthStatus{
			Type:   "synthetic",
			Status: health,
		}).
		WithOwner(owner).
		Write(t, client)
}
