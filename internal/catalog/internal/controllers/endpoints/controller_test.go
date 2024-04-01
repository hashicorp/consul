// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

var (
	badId = rtest.Resource(&pbresource.Type{Group: "not", Kind: "found", GroupVersion: "vfake"}, "foo").ID()
)

func TestWorkloadsToEndpoints(t *testing.T) {
	// This test's purpose is to ensure that converting multiple workloads to endpoints
	// happens as expected. It is not concerned with the data in each endpoint but rather
	// the removal of unconvertable workloads (nil endpoints returned by workloadToEndpoint).

	// The workload to endpoint conversion only cares about the service ports
	service := &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "http2", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
		},
	}

	workloadAddresses := []*pbcatalog.WorkloadAddress{
		{Host: "127.0.0.1"},
	}

	// This workload is port-matched with the service and should show up as an
	// endpoint in the final set.
	workloadData1 := &pbcatalog.Workload{
		Addresses: workloadAddresses,
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http2": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
		},
	}

	// This workload is NOT port-matched with the service and should be omitted.
	workloadData2 := &pbcatalog.Workload{
		Addresses: workloadAddresses,
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
	}

	// Build out the workloads.
	workloads := []*DecodedWorkload{
		rtest.MustDecode[*pbcatalog.Workload](
			t,
			rtest.Resource(pbcatalog.WorkloadType, "foo").
				WithData(t, workloadData1).
				Build()),

		rtest.MustDecode[*pbcatalog.Workload](
			t,
			rtest.Resource(pbcatalog.WorkloadType, "bar").
				WithData(t, workloadData2).
				Build()),
	}

	endpoints := workloadsToEndpoints(service, workloads)
	require.Len(t, endpoints.Endpoints, 1)
	prototest.AssertDeepEqual(t, workloads[0].Id, endpoints.Endpoints[0].TargetRef)
}

func TestWorkloadToEndpoint(t *testing.T) {
	// This test handles ensuring that the bulk of the functionality of
	// the workloadToEndpoint function works correctly.
	//
	// * WorkloadPorts that are not selected by one service port are ignored
	//   and not present in the resulting Endpoint
	// * WorkloadPorts that have a protocol mismatch with the service port
	//   are ignored and not present in the resulting Endpoint
	// * WorkloadAddresses with 0 non-ignored ports are omitted from the
	//   resulting Endpoint.
	// * Specifying no ports for a WorkloadAddress will use all the non-ignored
	//   ports. These are explicitly set but that is intended to be an
	//   implementation detail at this point.

	service := &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			// the workload will not have this port so it should be ignored
			{TargetPort: "not-found", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			// the workload will have a different protocol for this port and so it
			// will be ignored.
			{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
		},
	}

	workload := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			// this address will be in the endpoint with all the ports that are
			// not filtered out - so just http
			{Host: "127.0.0.1"},
			// this address will be in the endpoint but with a filtered ports list
			{Host: "198.18.1.1", Ports: []string{"http", "grpc"}},
			// this address should not show up in the endpoint because the port it
			// uses is filtered out
			{Host: "198.8.0.1", Ports: []string{"grpc"}},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			// the protocol is wrong here so it will not show up in the endpoints.
			"grpc": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
		},
		Identity: "test-identity",
		Dns: &pbcatalog.DNSPolicy{
			Weights: &pbcatalog.Weights{
				Passing: 3,
				Warning: 2,
			},
		},
	}

	data := rtest.MustDecode[*pbcatalog.Workload](t, rtest.Resource(pbcatalog.WorkloadType, "foo").
		WithData(t, workload).
		Build())

	expected := &pbcatalog.Endpoint{
		TargetRef: data.Id,
		Addresses: []*pbcatalog.WorkloadAddress{
			{Host: "127.0.0.1", Ports: []string{"http"}},
			{Host: "198.18.1.1", Ports: []string{"http"}},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": workload.Ports["http"],
		},
		// The health is critical because we are not setting the workload's
		// health status. The tests for determineWorkloadHealth will ensure
		// that we can properly determine the health status and the overall
		// controller tests will prove that the integration works as expected.
		HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
		Identity:     workload.Identity,
		Dns: &pbcatalog.DNSPolicy{
			Weights: &pbcatalog.Weights{
				Passing: 3,
				Warning: 2,
			},
		},
	}

	prototest.AssertDeepEqual(t, expected, workloadToEndpoint(service, data))
}

func TestWorkloadToEndpoint_AllAddressesFiltered(t *testing.T) {
	// This test checks the specific case where the workload has no
	// address/port combinations that remain unfiltered. In this
	// case we want to ensure nil is returned instead of an Endpoint
	// with no addresses.

	service := &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "not-found", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
	}

	workload := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{Host: "127.0.0.1"},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
	}

	data := rtest.MustDecode[*pbcatalog.Workload](
		t,
		rtest.Resource(pbcatalog.WorkloadType, "foo").
			WithData(t, workload).
			Build())

	require.Nil(t, workloadToEndpoint(service, data))
}

func TestWorkloadToEndpoint_MissingWorkloadProtocol(t *testing.T) {
	// This test checks that when a workload is missing its protocol,
	// we will default to service's protocol.

	service := &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "test-port", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
	}

	workload := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{Host: "127.0.0.1"},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"test-port": {Port: 8080},
		},
	}

	data := rtest.MustDecode[*pbcatalog.Workload](
		t,
		rtest.Resource(pbcatalog.WorkloadType, "foo").
			WithData(t, workload).
			Build())

	expected := &pbcatalog.Endpoint{
		TargetRef: data.Id,
		Addresses: []*pbcatalog.WorkloadAddress{
			{Host: "127.0.0.1", Ports: []string{"test-port"}},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"test-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
		// The health is critical because we are not setting the workload's
		// health status. The tests for determineWorkloadHealth will ensure
		// that we can properly determine the health status and the overall
		// controller tests will prove that the integration works as expected.
		HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
		Identity:     workload.Identity,
	}

	prototest.AssertDeepEqual(t, expected, workloadToEndpoint(service, data))
}

func TestServiceUnderManagement(t *testing.T) {
	// This test ensures that we can properly detect when a service
	// should have endpoints generated for it vs when those endpoints
	// are not being automatically managed.

	type testCase struct {
		svc     *pbcatalog.Service
		managed bool
	}

	cases := map[string]testCase{
		"nil": {
			svc:     nil,
			managed: false,
		},
		"nil-selector": {
			svc:     &pbcatalog.Service{Workloads: nil},
			managed: false,
		},
		"empty-selector": {
			svc:     &pbcatalog.Service{Workloads: &pbcatalog.WorkloadSelector{}},
			managed: false,
		},
		"exact-match": {
			svc: &pbcatalog.Service{Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{"foo"},
			}},
			managed: true,
		},
		"prefix-match": {
			svc: &pbcatalog.Service{Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo"},
			}},
			managed: true,
		},
		"multiple": {
			svc: &pbcatalog.Service{Workloads: &pbcatalog.WorkloadSelector{
				Names:    []string{"foo"},
				Prefixes: []string{"api-"},
			}},
			managed: true,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.managed, serviceUnderManagement(tcase.svc))
		})
	}
}

func TestDetermineWorkloadHealth(t *testing.T) {
	// This test ensures that parsing workload health out of the
	// resource status works as expected.

	type testCase struct {
		res      *pbresource.Resource
		expected pbcatalog.Health
	}

	cases := map[string]testCase{
		"no-status": {
			res:      rtest.Resource(pbcatalog.WorkloadType, "foo").Build(),
			expected: pbcatalog.Health_HEALTH_CRITICAL,
		},
		"condition-not-found": {
			res: rtest.Resource(pbcatalog.WorkloadType, "foo").
				WithStatus(workloadhealth.ControllerID, &pbresource.Status{
					Conditions: []*pbresource.Condition{
						{
							Type:   "other",
							State:  pbresource.Condition_STATE_TRUE,
							Reason: "NOT_RELEVANT",
						},
					},
				}).
				Build(),
			expected: pbcatalog.Health_HEALTH_CRITICAL,
		},
		"invalid-reason": {
			res: rtest.Resource(pbcatalog.WorkloadType, "foo").
				WithStatus(workloadhealth.ControllerID, &pbresource.Status{
					Conditions: []*pbresource.Condition{
						{
							Type:   workloadhealth.StatusConditionHealthy,
							State:  pbresource.Condition_STATE_TRUE,
							Reason: "INVALID_HEALTH_STATUS_REASON",
						},
					},
				}).
				Build(),
			expected: pbcatalog.Health_HEALTH_CRITICAL,
		},
		"passing": {
			res: rtest.Resource(pbcatalog.WorkloadType, "foo").
				WithStatus(workloadhealth.ControllerID, &pbresource.Status{
					Conditions: []*pbresource.Condition{
						{
							Type:   workloadhealth.StatusConditionHealthy,
							State:  pbresource.Condition_STATE_TRUE,
							Reason: pbcatalog.Health_HEALTH_PASSING.String(),
						},
					},
				}).
				Build(),
			expected: pbcatalog.Health_HEALTH_PASSING,
		},
		"warning": {
			res: rtest.Resource(pbcatalog.WorkloadType, "foo").
				WithStatus(workloadhealth.ControllerID, &pbresource.Status{
					Conditions: []*pbresource.Condition{
						{
							Type:   workloadhealth.StatusConditionHealthy,
							State:  pbresource.Condition_STATE_TRUE,
							Reason: pbcatalog.Health_HEALTH_WARNING.String(),
						},
					},
				}).
				Build(),
			expected: pbcatalog.Health_HEALTH_WARNING,
		},
		"critical": {
			res: rtest.Resource(pbcatalog.WorkloadType, "foo").
				WithStatus(workloadhealth.ControllerID, &pbresource.Status{
					Conditions: []*pbresource.Condition{
						{
							Type:   workloadhealth.StatusConditionHealthy,
							State:  pbresource.Condition_STATE_TRUE,
							Reason: pbcatalog.Health_HEALTH_CRITICAL.String(),
						},
					},
				}).
				Build(),
			expected: pbcatalog.Health_HEALTH_CRITICAL,
		},
		"maintenance": {
			res: rtest.Resource(pbcatalog.WorkloadType, "foo").
				WithStatus(workloadhealth.ControllerID, &pbresource.Status{
					Conditions: []*pbresource.Condition{
						{
							Type:   workloadhealth.StatusConditionHealthy,
							State:  pbresource.Condition_STATE_TRUE,
							Reason: pbcatalog.Health_HEALTH_MAINTENANCE.String(),
						},
					},
				}).
				Build(),
			expected: pbcatalog.Health_HEALTH_MAINTENANCE,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.expected, determineWorkloadHealth(tcase.res))
		})
	}
}

func TestWorkloadIdentityStatusFromEndpoints(t *testing.T) {
	cases := map[string]struct {
		endpoints *pbcatalog.ServiceEndpoints
		expStatus *pbresource.Condition
	}{
		"endpoints are nil": {
			expStatus: ConditionIdentitiesNotFound,
		},
		"endpoints without identities": {
			endpoints: &pbcatalog.ServiceEndpoints{},
			expStatus: ConditionIdentitiesNotFound,
		},
		"endpoints with identities": {
			endpoints: &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					{
						Identity: "foo",
					},
				},
			},
			expStatus: ConditionIdentitiesFound([]string{"foo"}),
		},
		"endpoints with multiple identities": {
			endpoints: &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					{
						Identity: "foo",
					},
					{
						Identity: "bar",
					},
				},
			},
			expStatus: ConditionIdentitiesFound([]string{"bar", "foo"}),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			prototest.AssertDeepEqual(t, c.expStatus, workloadIdentityStatusFromEndpoints(c.endpoints))
		})
	}
}

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	ctl       *controller.TestController
	tenancies []*pbresource.Tenancy
}

func (suite *controllerSuite) SetupTest() {
	suite.tenancies = resourcetest.TestTenancies()
	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancies...).
		Run(suite.T())
	suite.ctl = controller.NewTestController(ServiceEndpointsController(), client).
		WithLogger(testutil.Logger(suite.T()))
	suite.rt = suite.ctl.Runtime()
	suite.client = rtest.NewClient(suite.rt.Client)
}

func (suite *controllerSuite) requireEndpoints(resource *pbresource.Resource, expected ...*pbcatalog.Endpoint) {
	var svcEndpoints pbcatalog.ServiceEndpoints
	require.NoError(suite.T(), resource.Data.UnmarshalTo(&svcEndpoints))
	require.Len(suite.T(), svcEndpoints.Endpoints, len(expected))
	prototest.AssertElementsMatch(suite.T(), expected, svcEndpoints.Endpoints)
}

func (suite *controllerSuite) TestReconcile_ServiceNotFound() {
	// This test really only checks that the Reconcile call will not panic or otherwise error
	// when the request is for an endpoints object whose corresponding service does not exist.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		id := rtest.Resource(pbcatalog.ServiceEndpointsType, "not-found").WithTenancy(tenancy).ID()

		// Because the endpoints don't exist, this reconcile call not error but also shouldn't do anything useful.
		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
		require.NoError(suite.T(), err)
	})
}

func (suite *controllerSuite) TestReconcile_NoSelector_NoEndpoints() {
	// This test's purpose is to ensure that the service's status is
	// updated to record that its endpoints are not being automatically
	// managed. Additionally, with no endpoints pre-existing it will
	// not attempt to delete them.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		service := rtest.Resource(pbcatalog.ServiceType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		endpointsID := rtest.Resource(pbcatalog.ServiceEndpointsType, "test").WithTenancy(tenancy).ID()

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: endpointsID})
		require.NoError(suite.T(), err)

		suite.client.RequireStatusCondition(suite.T(), service.Id, ControllerID, ConditionUnmanaged)
	})
}

func (suite *controllerSuite) TestReconcile_NoSelector_ManagedEndpoints() {
	// This test's purpose is to ensure that when moving from managed endpoints
	// to unmanaged endpoints for a service, any already generated managed endpoints
	// get deleted.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		service := rtest.Resource(pbcatalog.ServiceType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		endpoints := rtest.Resource(pbcatalog.ServiceEndpointsType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.ServiceEndpoints{}).
			// this marks these endpoints as under management
			WithMeta(endpointsMetaManagedBy, ControllerID).
			Write(suite.T(), suite.client)

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: endpoints.Id})
		require.NoError(suite.T(), err)
		// the status should indicate the services endpoints are not being managed
		suite.client.RequireStatusCondition(suite.T(), service.Id, ControllerID, ConditionUnmanaged)
		// endpoints under management should be deleted
		suite.client.RequireResourceNotFound(suite.T(), endpoints.Id)
	})
}

func (suite *controllerSuite) TestReconcile_NoSelector_UnmanagedEndpoints() {
	// This test's purpose is to ensure that when re-reconciling a service that
	// doesn't have its endpoints managed, that we do not delete any unmanaged
	// ServiceEndpoints resource that the user would have manually written.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		service := rtest.Resource(pbcatalog.ServiceType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		endpoints := rtest.Resource(pbcatalog.ServiceEndpointsType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.ServiceEndpoints{}).
			Write(suite.T(), suite.client)

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: endpoints.Id})
		require.NoError(suite.T(), err)
		// the status should indicate the services endpoints are not being managed
		suite.client.RequireStatusCondition(suite.T(), service.Id, ControllerID, ConditionUnmanaged)
		// unmanaged endpoints should not be deleted when the service is unmanaged
		suite.client.RequireResourceExists(suite.T(), endpoints.Id)
	})
}

func (suite *controllerSuite) TestReconcile_Managed_NoPreviousEndpoints() {
	// This test's purpose is to ensure the managed endpoint generation occurs
	// as expected when there are no pre-existing endpoints.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		service := rtest.Resource(pbcatalog.ServiceType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{""},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		endpointsID := rtest.Resource(pbcatalog.ServiceEndpointsType, "test").WithTenancy(tenancy).ID()

		rtest.Resource(pbcatalog.WorkloadType, "test-workload").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "127.0.0.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: endpointsID})
		require.NoError(suite.T(), err)

		// Verify that the services status has been set to indicate endpoints are automatically managed.
		suite.client.RequireStatusCondition(suite.T(), service.Id, ControllerID, ConditionManaged)

		// The service endpoints metadata should include our tag to indcate it was generated by this controller
		res := suite.client.RequireResourceMeta(suite.T(), endpointsID, endpointsMetaManagedBy, ControllerID)

		var endpoints pbcatalog.ServiceEndpoints
		err = res.Data.UnmarshalTo(&endpoints)
		require.NoError(suite.T(), err)
		require.Len(suite.T(), endpoints.Endpoints, 1)
	})
	// We are not going to retest that the workloads to endpoints conversion process
	// The length check should be sufficient to prove the endpoints are being
	// converted. The unit tests for the workloadsToEndpoints functions prove that
	// the process works correctly in all cases.
}

func (suite *controllerSuite) TestReconcile_Managed_ExistingEndpoints() {
	// This test's purpose is to ensure that when the current set of endpoints
	// differs from any prior set of endpoints that the resource gets rewritten.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		service := rtest.Resource(pbcatalog.ServiceType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{""},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		endpoints := rtest.Resource(pbcatalog.ServiceEndpointsType, "test").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.ServiceEndpoints{}).
			WithOwner(service.Id).
			Write(suite.T(), suite.client)

		rtest.Resource(pbcatalog.WorkloadType, "test-workload").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "127.0.0.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: endpoints.Id})
		require.NoError(suite.T(), err)

		suite.client.RequireStatusCondition(suite.T(), service.Id, ControllerID, ConditionManaged)
		res := suite.client.RequireResourceMeta(suite.T(), endpoints.Id, endpointsMetaManagedBy, ControllerID)

		var newEndpoints pbcatalog.ServiceEndpoints
		err = res.Data.UnmarshalTo(&newEndpoints)
		require.NoError(suite.T(), err)
		require.Len(suite.T(), newEndpoints.Endpoints, 1)
	})
}

func (suite *controllerSuite) TestController() {
	// This test's purpose is to exercise the controller in a halfway realistic way.
	// Generally we are trying to go through the whole lifecycle of creating services,
	// adding workloads, modifying workload health and modifying the service selection
	// criteria. This isn't a full integration test as that would require also
	// executing the workload health controller. Instead workload health status is
	// synthesized as necessary.

	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(ServiceEndpointsController())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Add a service - there are no workloads so an empty endpoints
		// object should be created.
		service := rtest.Resource(pbcatalog.ServiceType, "api").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		// Wait for the controller to record that the endpoints are being managed
		res := suite.client.WaitForReconciliation(suite.T(), service.Id, ControllerID)
		// Check that the services status was updated accordingly
		rtest.RequireStatusCondition(suite.T(), res, ControllerID, ConditionManaged)
		rtest.RequireStatusCondition(suite.T(), res, ControllerID, ConditionIdentitiesNotFound)

		// Check that the endpoints resource exists and contains 0 endpoints
		endpointsID := rtest.Resource(pbcatalog.ServiceEndpointsType, "api").WithTenancy(tenancy).ID()
		endpoints := suite.client.RequireResourceExists(suite.T(), endpointsID)
		suite.requireEndpoints(endpoints)

		// Now add a workload that would be selected by the service. Leave
		// the workload in a state where its health has not been reconciled
		workload := rtest.Resource(pbcatalog.WorkloadType, "api-1").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "127.0.0.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
				Identity: "api",
			}).
			Write(suite.T(), suite.client)

		suite.client.WaitForStatusCondition(suite.T(), service.Id, ControllerID,
			ConditionIdentitiesFound([]string{"api"}))

		// Wait for the endpoints to be regenerated
		endpoints = suite.client.WaitForNewVersion(suite.T(), endpointsID, endpoints.Version)

		// Verify that the generated endpoints now contain the workload
		suite.requireEndpoints(endpoints, &pbcatalog.Endpoint{
			TargetRef: workload.Id,
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1", Ports: []string{"http"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			Identity:     "api",
		})

		// Update the health status of the workload
		suite.client.WriteStatus(suite.ctx, &pbresource.WriteStatusRequest{
			Id:  workload.Id,
			Key: workloadhealth.ControllerID,
			Status: &pbresource.Status{
				ObservedGeneration: workload.Generation,
				Conditions: []*pbresource.Condition{
					{
						Type:   workloadhealth.StatusConditionHealthy,
						State:  pbresource.Condition_STATE_TRUE,
						Reason: "HEALTH_PASSING",
					},
				},
			},
		})

		// Wait for the endpoints to be regenerated
		endpoints = suite.client.WaitForNewVersion(suite.T(), endpointsID, endpoints.Version)

		// ensure the endpoint was put into the passing state
		suite.requireEndpoints(endpoints, &pbcatalog.Endpoint{
			TargetRef: workload.Id,
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1", Ports: []string{"http"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			Identity:     "api",
		})

		// Update workload identity and check that the status on the service is updated
		workload = rtest.Resource(pbcatalog.WorkloadType, "api-1").WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "127.0.0.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					"grpc": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
				Identity: "endpoints-api-identity",
			}).
			Write(suite.T(), suite.client)

		suite.client.WaitForStatusCondition(suite.T(), service.Id, ControllerID, ConditionIdentitiesFound([]string{"endpoints-api-identity"}))

		// Verify that the generated endpoints now contain the workload
		endpoints = suite.client.WaitForNewVersion(suite.T(), endpointsID, endpoints.Version)
		suite.requireEndpoints(endpoints, &pbcatalog.Endpoint{
			TargetRef: workload.Id,
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1", Ports: []string{"http"}},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			Identity:     "endpoints-api-identity",
		})

		// rewrite the service to add more selection criteria. This should trigger
		// reconciliation but shouldn't result in updating the endpoints because
		// the actual list of currently selected workloads has not changed
		rtest.Resource(pbcatalog.ServiceType, "api").WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
					Names:    []string{"doesnt-matter"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		// Wait for the service status' observed generation to get bumped
		service = suite.client.WaitForReconciliation(suite.T(), service.Id, ControllerID)

		// Verify that the endpoints were not regenerated
		suite.client.RequireVersionUnchanged(suite.T(), endpointsID, endpoints.Version)

		// Update the service.
		updatedService := rtest.Resource(pbcatalog.ServiceType, "api").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
			}).
			Write(suite.T(), suite.client)

		// Wait for the endpoints to be regenerated
		endpoints = suite.client.WaitForNewVersion(suite.T(), endpointsID, endpoints.Version)
		rtest.RequireOwner(suite.T(), endpoints, updatedService.Id, false)

		// Delete the endpoints. The controller should bring these back momentarily
		suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: endpointsID})

		// Wait for controller to recreate the endpoints
		retry.Run(suite.T(), func(r *retry.R) {
			suite.client.RequireResourceExists(r, endpointsID)
		})

		// Move the service to having unmanaged endpoints
		rtest.Resource(pbcatalog.ServiceType, "api").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}).
			Write(suite.T(), suite.client)

		res = suite.client.WaitForReconciliation(suite.T(), service.Id, ControllerID)
		rtest.RequireStatusCondition(suite.T(), res, ControllerID, ConditionUnmanaged)

		// Verify that the endpoints were deleted
		suite.client.RequireResourceNotFound(suite.T(), endpointsID)
	})
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func (suite *controllerSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			testFunc(tenancy)
		})
	}
}

func (suite *controllerSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}
