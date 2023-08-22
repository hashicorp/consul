// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type xdsControllerTestSuite struct {
	suite.Suite

	ctx     context.Context
	client  *resourcetest.Client
	runtime controller.Runtime

	ctl     *xdsReconciler
	mapper  *bimapper.Mapper
	updater *mockUpdater
	fetcher TrustBundleFetcher

	fooProxyStateTemplate          *pbresource.Resource
	barProxyStateTemplate          *pbresource.Resource
	barEndpointRefs                map[string]*pbproxystate.EndpointRef
	fooEndpointRefs                map[string]*pbproxystate.EndpointRef
	fooEndpoints                   *pbresource.Resource
	fooService                     *pbresource.Resource
	fooBarEndpoints                *pbresource.Resource
	fooBarService                  *pbresource.Resource
	expectedFooProxyStateEndpoints map[string]*pbproxystate.Endpoints
	expectedBarProxyStateEndpoints map[string]*pbproxystate.Endpoints
	expectedTrustBundle            map[string]*pbproxystate.TrustBundle
}

func (suite *xdsControllerTestSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	resourceClient := svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.client = resourcetest.NewClient(resourceClient)
	suite.fetcher = mockFetcher

	suite.mapper = bimapper.New(types.ProxyStateTemplateType, catalog.ServiceEndpointsType)
	suite.updater = NewMockUpdater()

	suite.ctl = &xdsReconciler{
		bimapper:         suite.mapper,
		updater:          suite.updater,
		fetchTrustBundle: suite.fetcher,
	}
}

func mockFetcher() (*pbproxystate.TrustBundle, error) {
	var bundle pbproxystate.TrustBundle
	bundle = pbproxystate.TrustBundle{
		TrustDomain: "some-trust-domain",
		Roots:       []string{"some-root", "some-other-root"},
	}
	return &bundle, nil
}

// This test ensures when a ProxyState is deleted, it is no longer tracked in the mapper.
func (suite *xdsControllerTestSuite) TestReconcile_NoProxyStateTemplate() {
	// Track the id of a non-existent ProxyStateTemplate.
	proxyStateTemplateId := resourcetest.Resource(types.ProxyStateTemplateType, "not-found").ID()
	suite.mapper.TrackItem(proxyStateTemplateId, []resource.ReferenceOrID{})

	// Run the reconcile, and since no ProxyStateTemplate is stored, this simulates a deletion.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: proxyStateTemplateId,
	})
	require.NoError(suite.T(), err)

	// Assert that nothing is tracked in the mapper.
	require.True(suite.T(), suite.mapper.IsEmpty())
}

// This test ensures if the controller was previously tracking a ProxyStateTemplate, and now that proxy has
// disconnected from this server, it's ignored and removed from the mapper.
func (suite *xdsControllerTestSuite) TestReconcile_RemoveTrackingProxiesNotConnectedToServer() {
	// Store the initial ProxyStateTemplate and track it in the mapper.
	proxyStateTemplate := resourcetest.Resource(types.ProxyStateTemplateType, "test").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{}).
		Write(suite.T(), suite.client)

	suite.mapper.TrackItem(proxyStateTemplate.Id, []resource.ReferenceOrID{})

	// Simulate the proxy disconnecting from this server. The resource still exists, but this proxy might be connected
	// to a different server now, so we no longer need to track it.
	suite.updater.notConnected = true

	// Run the reconcile.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: proxyStateTemplate.Id,
	})
	require.NoError(suite.T(), err)

	// Assert that nothing is tracked in the mapper.
	require.True(suite.T(), suite.mapper.IsEmpty())
}

// This test sets up the updater to return an error when calling PushChange, and ensures the status is set
// correctly.
func (suite *xdsControllerTestSuite) TestReconcile_PushChangeError() {
	// Have the mock simulate an error from the PushChange call.
	suite.updater.pushChangeError = true

	// Setup a happy path scenario.
	suite.setupFooProxyStateTemplateAndEndpoints()

	// Run the reconcile.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: suite.fooProxyStateTemplate.Id,
	})
	require.Error(suite.T(), err)

	// Assert on the status reflecting endpoint not found.
	suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionRejectedPushChangeFailed(status.KeyFromID(suite.fooProxyStateTemplate.Id)))
}

// This test sets up a ProxyStateTemplate that references a ServiceEndpoints that doesn't exist, and ensures the
// status is correct.
func (suite *xdsControllerTestSuite) TestReconcile_MissingEndpoint() {
	// Set fooProxyStateTemplate with a reference to fooEndpoints, without storing fooEndpoints so the controller should
	// notice it's missing.
	fooEndpointsId := resourcetest.Resource(catalog.ServiceEndpointsType, "foo-service").ID()
	fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id:   fooEndpointsId,
		Port: "mesh",
	}

	fooProxyStateTemplate := resourcetest.Resource(types.ProxyStateTemplateType, "foo-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			RequiredEndpoints: fooRequiredEndpoints,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		Write(suite.T(), suite.client)

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, fooProxyStateTemplate.Id)
	})

	// Run the reconcile.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: fooProxyStateTemplate.Id,
	})
	require.Error(suite.T(), err)

	// Assert on the status reflecting endpoint not found.
	suite.client.RequireStatusCondition(suite.T(), fooProxyStateTemplate.Id, ControllerName, status.ConditionRejectedErrorReadingEndpoints(status.KeyFromID(fooEndpointsId), "rpc error: code = NotFound desc = resource not found"))
}

// This test sets up a ProxyStateTemplate that references a ServiceEndpoints that can't be read correctly, and
// checks the status is correct.
func (suite *xdsControllerTestSuite) TestReconcile_ReadEndpointError() {
	badID := &pbresource.ID{
		Type: &pbresource.Type{
			Group:        "not",
			Kind:         "found",
			GroupVersion: "vfake",
		},
		Tenancy: &pbresource.Tenancy{Namespace: "default", Partition: "default", PeerName: "local"},
	}
	fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id:   badID,
		Port: "mesh",
	}

	fooProxyStateTemplate := resourcetest.Resource(types.ProxyStateTemplateType, "foo-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			RequiredEndpoints: fooRequiredEndpoints,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		Write(suite.T(), suite.client)

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, fooProxyStateTemplate.Id)
	})

	// Run the reconcile.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: fooProxyStateTemplate.Id,
	})
	require.Error(suite.T(), err)

	// Assert on the status reflecting endpoint couldn't be read.
	suite.client.RequireStatusCondition(suite.T(), fooProxyStateTemplate.Id, ControllerName, status.ConditionRejectedErrorReadingEndpoints(status.KeyFromID(badID), "rpc error: code = InvalidArgument desc = id.name is required"))
}

// This test is a happy path creation test to make sure pbproxystate.Endpoints are created in the computed
// pbmesh.ProxyState from the RequiredEndpoints references. More specific translations between endpoint references
// and pbproxystate.Endpoints are unit tested in endpoint_builder.go.
func (suite *xdsControllerTestSuite) TestReconcile_ProxyStateTemplateComputesEndpoints() {
	// Set up fooEndpoints and fooProxyStateTemplate with a reference to fooEndpoints and store them in the state store.
	// This setup saves expected values in the suite so it can be asserted against later.
	suite.setupFooProxyStateTemplateAndEndpoints()

	// Run the reconcile.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: suite.fooProxyStateTemplate.Id,
	})
	require.NoError(suite.T(), err)

	// Assert on the status.
	suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())

	// Assert that the endpoints computed in the controller matches the expected endpoints.
	actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
	prototest.AssertDeepEqual(suite.T(), suite.expectedFooProxyStateEndpoints, actualEndpoints)
}

func (suite *xdsControllerTestSuite) TestReconcile_ProxyStateTemplateSetsTrustBundles() {
	// This test is a happy path creation test to make sure pbproxystate.Template.TrustBundles are created in the computed
	// pbmesh.ProxyState from the TrustBundleFetcher.
	suite.setupFooProxyStateTemplateAndEndpoints()

	// Run the reconcile.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: suite.fooProxyStateTemplate.Id,
	})
	require.NoError(suite.T(), err)

	// Assert on the status.
	suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())

	// Assert that the endpoints computed in the controller matches the expected endpoints.
	actualTrustBundle := suite.updater.GetTrustBundle(suite.fooProxyStateTemplate.Id.Name)
	prototest.AssertDeepEqual(suite.T(), suite.expectedTrustBundle, actualTrustBundle)
}

// This test is a happy path creation test that calls reconcile multiple times with a more complex setup. This
// scenario is trickier to test in the controller test because the end computation of the xds controller is not
// stored in the state store. So this test ensures that between multiple reconciles the correct ProxyStates are
// computed for each ProxyStateTemplate.
func (suite *xdsControllerTestSuite) TestReconcile_MultipleProxyStateTemplatesComputesMultipleEndpoints() {
	// Set up fooProxyStateTemplate and barProxyStateTemplate and their associated resources and store them. Resources
	// and expected results are stored in the suite to assert against.
	suite.setupFooBarProxyStateTemplateAndEndpoints()

	// Reconcile the fooProxyStateTemplate.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: suite.fooProxyStateTemplate.Id,
	})
	require.NoError(suite.T(), err)

	// Assert on the status.
	suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())

	// Assert that the endpoints computed in the controller matches the expected endpoints.
	actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
	prototest.AssertDeepEqual(suite.T(), suite.expectedFooProxyStateEndpoints, actualEndpoints)

	// Reconcile the barProxyStateTemplate.
	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: suite.barProxyStateTemplate.Id,
	})
	require.NoError(suite.T(), err)

	// Assert on the status.
	suite.client.RequireStatusCondition(suite.T(), suite.barProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())

	// Assert that the endpoints computed in the controller matches the expected endpoints.
	actualBarEndpoints := suite.updater.GetEndpoints(suite.barProxyStateTemplate.Id.Name)
	prototest.AssertDeepEqual(suite.T(), suite.expectedBarProxyStateEndpoints, actualBarEndpoints)
}

// Sets up a full controller, and tests that reconciles are getting triggered for the events it should.
func (suite *xdsControllerTestSuite) TestController_ComputeAddUpdateEndpoints() {
	// Run the controller manager.
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)
	mgr.Register(Controller(suite.mapper, suite.updater, suite.fetcher))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	// Set up fooEndpoints and fooProxyStateTemplate with a reference to fooEndpoints. These need to be stored
	// because the controller reconcile looks them up.
	suite.setupFooProxyStateTemplateAndEndpoints()

	// Assert that the expected ProxyState matches the actual ProxyState that PushChange was called with. This needs to
	// be in a retry block unlike the Reconcile tests because the controller triggers asynchronously.
	retry.Run(suite.T(), func(r *retry.R) {
		actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
		// Assert on the status.
		suite.client.RequireStatusCondition(r, suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())
		// Assert that the endpoints computed in the controller matches the expected endpoints.
		prototest.AssertDeepEqual(r, suite.expectedFooProxyStateEndpoints, actualEndpoints)
	})

	// Now, update the endpoint to be unhealthy. This will ensure the controller is getting triggered on changes to this
	// endpoint that it should be tracking, even when the ProxyStateTemplate does not change.
	resourcetest.Resource(catalog.ServiceEndpointsType, "foo-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.1.1.1",
						Ports: []string{"mesh"},
					},
					{
						Host:  "10.2.2.2",
						Ports: []string{"mesh"},
					},
				},
				HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
			},
		}}).
		WithOwner(suite.fooService.Id).
		Write(suite.T(), suite.client)

	// Wait for the endpoint to be written.
	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireVersionChanged(suite.T(), suite.fooEndpoints.Id, suite.fooEndpoints.Version)
	})

	// Update the expected endpoints to also have unhealthy status.
	suite.expectedFooProxyStateEndpoints["test-cluster-1"].Endpoints[0].HealthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY
	suite.expectedFooProxyStateEndpoints["test-cluster-1"].Endpoints[1].HealthStatus = pbproxystate.HealthStatus_HEALTH_STATUS_UNHEALTHY

	retry.Run(suite.T(), func(r *retry.R) {
		actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
		// Assert on the status.
		suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())
		// Assert that the endpoints computed in the controller matches the expected endpoints.
		prototest.AssertDeepEqual(r, suite.expectedFooProxyStateEndpoints, actualEndpoints)
	})

	// Now add a new endpoint reference and endpoint to the fooProxyStateTemplate. This will ensure that the controller
	// now tracks the newly added endpoint.
	secondService := resourcetest.Resource(catalog.ServiceType, "second-service").
		WithData(suite.T(), &pbcatalog.Service{}).
		Write(suite.T(), suite.client)

	secondEndpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "second-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.5.5.5",
						Ports: []string{"mesh"},
					},
					{
						Host:  "10.6.6.6",
						Ports: []string{"mesh"},
					},
				},
			},
		}}).
		WithOwner(secondService.Id).
		Write(suite.T(), suite.client)

	// Update the endpoint references on the fooProxyStateTemplate.
	suite.fooEndpointRefs["test-cluster-2"] = &pbproxystate.EndpointRef{
		Id:   secondEndpoints.Id,
		Port: "mesh",
	}
	oldVersion := suite.fooProxyStateTemplate.Version
	fooProxyStateTemplate := resourcetest.Resource(types.ProxyStateTemplateType, "foo-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			RequiredEndpoints: suite.fooEndpointRefs,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		Write(suite.T(), suite.client)

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireVersionChanged(r, fooProxyStateTemplate.Id, oldVersion)
	})

	// Update the expected endpoints with this new endpoints.
	suite.expectedFooProxyStateEndpoints["test-cluster-2"] = &pbproxystate.Endpoints{
		Endpoints: []*pbproxystate.Endpoint{
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.5.5.5",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.6.6.6",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
		},
	}

	retry.Run(suite.T(), func(r *retry.R) {
		actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
		// Assert on the status.
		suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())
		// Assert that the endpoints computed in the controller matches the expected endpoints.
		prototest.AssertDeepEqual(r, suite.expectedFooProxyStateEndpoints, actualEndpoints)
	})

}

// Setup: fooProxyStateTemplate with an EndpointsRef to fooEndpoints
// Saves all related resources to the suite so they can be modified if needed.
func (suite *xdsControllerTestSuite) setupFooProxyStateTemplateAndEndpoints() {
	fooService := resourcetest.Resource(catalog.ServiceType, "foo-service").
		WithData(suite.T(), &pbcatalog.Service{}).
		Write(suite.T(), suite.client)

	fooEndpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "foo-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.1.1.1",
						Ports: []string{"mesh"},
					},
					{
						Host:  "10.2.2.2",
						Ports: []string{"mesh"},
					},
				},
			},
		}}).
		WithOwner(fooService.Id).
		Write(suite.T(), suite.client)

	fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id:   fooEndpoints.Id,
		Port: "mesh",
	}

	fooProxyStateTemplate := resourcetest.Resource(types.ProxyStateTemplateType, "foo-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			RequiredEndpoints: fooRequiredEndpoints,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		Write(suite.T(), suite.client)

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, fooProxyStateTemplate.Id)
	})

	expectedFooProxyStateEndpoints := map[string]*pbproxystate.Endpoints{
		"test-cluster-1": {Endpoints: []*pbproxystate.Endpoint{
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.1.1.1",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.2.2.2",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
		}},
	}

	expectedTrustBundle := map[string]*pbproxystate.TrustBundle{
		"local": {
			TrustDomain: "some-trust-domain",
			Roots:       []string{"some-root", "some-other-root"},
		},
	}

	suite.fooService = fooService
	suite.fooEndpoints = fooEndpoints
	suite.fooEndpointRefs = fooRequiredEndpoints
	suite.fooProxyStateTemplate = fooProxyStateTemplate
	suite.expectedFooProxyStateEndpoints = expectedFooProxyStateEndpoints
	suite.expectedTrustBundle = expectedTrustBundle
}

// Setup:
//   - fooProxyStateTemplate with an EndpointsRef to fooEndpoints and fooBarEndpoints.
//   - barProxyStateTemplate with an EndpointsRef to fooBarEndpoints.
//
// Saves all related resources to the suite so they can be modified if needed.
func (suite *xdsControllerTestSuite) setupFooBarProxyStateTemplateAndEndpoints() {
	fooService := resourcetest.Resource(catalog.ServiceType, "foo-service").
		WithData(suite.T(), &pbcatalog.Service{}).
		Write(suite.T(), suite.client)

	fooEndpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "foo-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.1.1.1",
						Ports: []string{"mesh"},
					},
					{
						Host:  "10.2.2.2",
						Ports: []string{"mesh"},
					},
				},
			},
		}}).
		WithOwner(fooService.Id).
		Write(suite.T(), suite.client)

	fooBarService := resourcetest.Resource(catalog.ServiceType, "foo-bar-service").
		WithData(suite.T(), &pbcatalog.Service{}).
		Write(suite.T(), suite.client)

	fooBarEndpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "foo-bar-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.3.3.3",
						Ports: []string{"mesh"},
					},
					{
						Host:  "10.4.4.4",
						Ports: []string{"mesh"},
					},
				},
			},
		}}).
		WithOwner(fooBarService.Id).
		Write(suite.T(), suite.client)

	fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id:   fooEndpoints.Id,
		Port: "mesh",
	}
	fooRequiredEndpoints["test-cluster-2"] = &pbproxystate.EndpointRef{
		Id:   fooBarEndpoints.Id,
		Port: "mesh",
	}

	barRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	barRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id: fooBarEndpoints.Id,
		// Sidecar proxy controller will usually set mesh port here.
		Port: "mesh",
	}

	fooProxyStateTemplate := resourcetest.Resource(types.ProxyStateTemplateType, "foo-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			// Contains the foo and foobar endpoints.
			RequiredEndpoints: fooRequiredEndpoints,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		Write(suite.T(), suite.client)

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, fooProxyStateTemplate.Id)
	})

	barProxyStateTemplate := resourcetest.Resource(types.ProxyStateTemplateType, "bar-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			// Contains the foobar endpoint.
			RequiredEndpoints: barRequiredEndpoints,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		Write(suite.T(), suite.client)

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, barProxyStateTemplate.Id)
	})

	expectedFooProxyStateEndpoints := map[string]*pbproxystate.Endpoints{
		"test-cluster-1": {Endpoints: []*pbproxystate.Endpoint{
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.1.1.1",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.2.2.2",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
		}},
		"test-cluster-2": {Endpoints: []*pbproxystate.Endpoint{
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.3.3.3",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.4.4.4",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
		}},
	}

	expectedBarProxyStateEndpoints := map[string]*pbproxystate.Endpoints{
		"test-cluster-1": {Endpoints: []*pbproxystate.Endpoint{
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.3.3.3",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
			{
				Address: &pbproxystate.Endpoint_HostPort{
					HostPort: &pbproxystate.HostPortAddress{
						Host: "10.4.4.4",
						Port: 20000,
					},
				},
				HealthStatus: pbproxystate.HealthStatus_HEALTH_STATUS_HEALTHY,
			},
		}},
	}

	suite.fooProxyStateTemplate = fooProxyStateTemplate
	suite.barProxyStateTemplate = barProxyStateTemplate
	suite.barEndpointRefs = barRequiredEndpoints
	suite.fooEndpointRefs = fooRequiredEndpoints
	suite.fooEndpoints = fooEndpoints
	suite.fooService = fooService
	suite.fooBarEndpoints = fooBarEndpoints
	suite.fooBarService = fooBarService
	suite.expectedFooProxyStateEndpoints = expectedFooProxyStateEndpoints
	suite.expectedBarProxyStateEndpoints = expectedBarProxyStateEndpoints
}

func TestXdsController(t *testing.T) {
	suite.Run(t, new(xdsControllerTestSuite))
}
