// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protojson"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	proxytracker "github.com/hashicorp/consul/internal/mesh/proxy-tracker"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
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

	ctl             *xdsReconciler
	mapper          *bimapper.Mapper
	updater         *mockUpdater
	fetcher         TrustBundleFetcher
	leafMapper      *LeafMapper
	leafCertManager *leafcert.Manager
	leafCancels     *LeafCancels
	leafCertEvents  chan controller.Event
	signer          *leafcert.TestSigner
	tenancies       []*pbresource.Tenancy

	fooProxyStateTemplate          *pbresource.Resource
	barProxyStateTemplate          *pbresource.Resource
	barEndpointRefs                map[string]*pbproxystate.EndpointRef
	fooEndpointRefs                map[string]*pbproxystate.EndpointRef
	fooLeafRefs                    map[string]*pbproxystate.LeafCertificateRef
	fooEndpoints                   *pbresource.Resource
	fooService                     *pbresource.Resource
	fooBarEndpoints                *pbresource.Resource
	fooBarService                  *pbresource.Resource
	expectedFooProxyStateEndpoints map[string]*pbproxystate.Endpoints
	expectedBarProxyStateEndpoints map[string]*pbproxystate.Endpoints
	expectedFooProxyStateSpiffes   map[string]string
	expectedTrustBundle            map[string]*pbproxystate.TrustBundle
}

func (suite *xdsControllerTestSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = resourcetest.TestTenancies()
	resourceClient := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.client = resourcetest.NewClient(resourceClient)
	suite.fetcher = mockFetcher

	suite.mapper = bimapper.New(pbmesh.ProxyStateTemplateType, pbcatalog.ServiceEndpointsType)
	suite.updater = newMockUpdater()

	suite.leafMapper = &LeafMapper{
		bimapper.New(pbmesh.ProxyStateTemplateType, InternalLeafType),
	}
	lcm, signer := leafcert.NewTestManager(suite.T(), nil)
	signer.UpdateCA(suite.T(), nil)
	suite.signer = signer
	suite.leafCertManager = lcm
	suite.leafCancels = &LeafCancels{
		Cancels: make(map[string]context.CancelFunc),
	}
	suite.leafCertEvents = make(chan controller.Event, 1000)

	suite.ctl = &xdsReconciler{
		endpointsMapper:  suite.mapper,
		updater:          suite.updater,
		fetchTrustBundle: suite.fetcher,
		leafMapper:       suite.leafMapper,
		leafCertManager:  suite.leafCertManager,
		leafCancels:      suite.leafCancels,
		leafCertEvents:   suite.leafCertEvents,
		datacenter:       "dc1",
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

// This test ensures when a ProxyState is deleted, it is no longer tracked in the mappers.
func (suite *xdsControllerTestSuite) TestReconcile_NoProxyStateTemplate() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Track the id of a non-existent ProxyStateTemplate.
		proxyStateTemplateId := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "not-found").WithTenancy(tenancy).ID()

		suite.T().Cleanup(suite.deleteResourceFunc(proxyStateTemplateId))

		suite.mapper.TrackItem(proxyStateTemplateId, []resource.ReferenceOrID{})
		suite.leafMapper.TrackItem(proxyStateTemplateId, []resource.ReferenceOrID{})
		require.False(suite.T(), suite.mapper.IsEmpty())
		require.False(suite.T(), suite.leafMapper.IsEmpty())

		// Run the reconcile, and since no ProxyStateTemplate is stored, this simulates a deletion.
		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: proxyStateTemplateId,
		})
		require.NoError(suite.T(), err)

		// Assert that nothing is tracked in the endpoints mapper.
		require.True(suite.T(), suite.mapper.IsEmpty())
		require.True(suite.T(), suite.leafMapper.IsEmpty())
	})
}

// This test ensures if the controller was previously tracking a ProxyStateTemplate, and now that proxy has
// disconnected from this server, it's ignored and removed from the mapper.
func (suite *xdsControllerTestSuite) TestReconcile_RemoveTrackingProxiesNotConnectedToServer() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Store the initial ProxyStateTemplate and track it in the mapper.
		proxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "test").
			WithData(suite.T(), &pbmesh.ProxyStateTemplate{}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(proxyStateTemplate.Id))

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
	})
}

// This test sets up the updater to return an error when calling PushChange, and ensures the status is set
// correctly.
func (suite *xdsControllerTestSuite) TestReconcile_PushChangeError() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Have the mock simulate an error from the PushChange call.
		suite.updater.pushChangeError = true

		// Setup a happy path scenario.
		suite.setupFooProxyStateTemplateWithReferences(tenancy)

		// Run the reconcile.
		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: suite.fooProxyStateTemplate.Id,
		})
		require.Error(suite.T(), err)

		// Assert on the status reflecting endpoint not found.
		suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionRejectedPushChangeFailed(status.KeyFromID(suite.fooProxyStateTemplate.Id)))
	})
}

// This test sets up a ProxyStateTemplate that references a ServiceEndpoints that doesn't exist, and ensures the
// status is correct.
func (suite *xdsControllerTestSuite) TestReconcile_MissingEndpoint() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Set fooProxyStateTemplate with a reference to fooEndpoints, without storing fooEndpoints so the controller should
		// notice it's missing.
		fooEndpointsId := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "foo-service").WithTenancy(tenancy).ID()
		fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
		fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
			Id:        fooEndpointsId,
			MeshPort:  "mesh",
			RoutePort: "http",
		}

		fooProxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "foo-pst").
			WithData(suite.T(), &pbmesh.ProxyStateTemplate{
				RequiredEndpoints: fooRequiredEndpoints,
				ProxyState:        &pbmesh.ProxyState{},
			}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(fooProxyStateTemplate.Id))

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
	})
}

// This test sets up a ProxyStateTemplate that references a ServiceEndpoints that can't be read correctly, and
// checks the status is correct.
func (suite *xdsControllerTestSuite) TestReconcile_ReadEndpointError() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		badID := &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "not",
				Kind:         "found",
				GroupVersion: "vfake",
			},
			Tenancy: tenancy,
		}
		fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
		fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
			Id:        badID,
			MeshPort:  "mesh",
			RoutePort: "http",
		}

		fooProxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "foo-pst").
			WithData(suite.T(), &pbmesh.ProxyStateTemplate{
				RequiredEndpoints: fooRequiredEndpoints,
				ProxyState:        &pbmesh.ProxyState{},
			}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(fooProxyStateTemplate.Id))

		retry.Run(suite.T(), func(r *retry.R) {
			suite.client.RequireResourceExists(r, fooProxyStateTemplate.Id)
		})

		// Run the reconcile.
		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: fooProxyStateTemplate.Id,
		})
		require.Error(suite.T(), err)

		// Assert on the status reflecting endpoint couldn't be read.
		suite.client.RequireStatusCondition(suite.T(), fooProxyStateTemplate.Id, ControllerName, status.ConditionRejectedErrorReadingEndpoints(
			status.KeyFromID(badID),
			"rpc error: code = InvalidArgument desc = id.name invalid: a resource name must consist of lower case alphanumeric characters or '-', must start and end with an alphanumeric character and be less than 64 characters, got: \"\"",
		))
	})
}

// This test is a happy path creation test to make sure pbproxystate.Endpoints are created in the computed
// pbmesh.ProxyState from the RequiredEndpoints references. More specific translations between endpoint references
// and pbproxystate.Endpoints are unit tested in endpoint_builder.go.
func (suite *xdsControllerTestSuite) TestReconcile_ProxyStateTemplateComputesEndpoints() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Set up fooEndpoints and fooProxyStateTemplate with a reference to fooEndpoints and store them in the state store.
		// This setup saves expected values in the suite so it can be asserted against later.
		suite.setupFooProxyStateTemplateWithReferences(tenancy)

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
	})
}

func (suite *xdsControllerTestSuite) TestReconcile_ProxyStateTemplateComputesLeafCerts() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Set up fooEndpoints and fooProxyStateTemplate with a reference to fooEndpoints and store them in the state store.
		// This setup saves expected values in the suite so it can be asserted against later.
		suite.setupFooProxyStateTemplateWithReferences(tenancy)

		// Run the reconcile.
		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: suite.fooProxyStateTemplate.Id,
		})
		require.NoError(suite.T(), err)

		// Assert on the status.
		suite.client.RequireStatusCondition(suite.T(), suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())

		// Assert that the actual leaf certs computed are match the expected leaf cert spiffes.
		actualLeafs := suite.updater.GetLeafs(suite.fooProxyStateTemplate.Id.Name)

		for k, l := range actualLeafs {
			pemDecode, _ := pem.Decode([]byte(l.Cert))
			cert, err := x509.ParseCertificate(pemDecode.Bytes)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), cert.URIs[0].String(), suite.expectedFooProxyStateSpiffes[k])
		}
	})
}

// This test is a happy path creation test to make sure pbproxystate.Template.TrustBundles are created in the computed
// pbmesh.ProxyState from the TrustBundleFetcher.
func (suite *xdsControllerTestSuite) TestReconcile_ProxyStateTemplateSetsTrustBundles() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		suite.setupFooProxyStateTemplateWithReferences(tenancy)

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
	})
}

// This test is a happy path creation test that calls reconcile multiple times with a more complex setup. This
// scenario is trickier to test in the controller test because the end computation of the xds controller is not
// stored in the state store. So this test ensures that between multiple reconciles the correct ProxyStates are
// computed for each ProxyStateTemplate.
func (suite *xdsControllerTestSuite) TestReconcile_MultipleProxyStateTemplatesComputesMultipleEndpoints() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Set up fooProxyStateTemplate and barProxyStateTemplate and their associated resources and store them. Resources
		// and expected results are stored in the suite to assert against.
		suite.setupFooBarProxyStateTemplateAndEndpoints(tenancy)

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
	})
}

// Sets up a full controller, and tests that reconciles are getting triggered for the events it should.
func (suite *xdsControllerTestSuite) TestController_ComputeAddUpdateEndpointReferences() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Run the controller manager.
		mgr := controller.NewManager(suite.client, suite.runtime.Logger)
		mgr.Register(Controller(suite.mapper, suite.updater, suite.fetcher, suite.leafCertManager, suite.leafMapper, suite.leafCancels, "dc1"))
		mgr.SetRaftLeader(true)
		go mgr.Run(suite.ctx)

		suite.setupFooProxyStateTemplateWithReferences(tenancy)

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
		svc := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "foo-service").
			WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
				{
					Ports: map[string]*pbcatalog.WorkloadPort{
						"mesh": {
							Port:     20000,
							Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
						},
						"http": {
							Port:     8080,
							Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
						},
					},
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host:  "10.1.1.1",
							Ports: []string{"mesh", "http"},
						},
						{
							Host:  "10.2.2.2",
							Ports: []string{"mesh", "http"},
						},
					},
					HealthStatus: pbcatalog.Health_HEALTH_CRITICAL,
				},
			}}).
			WithOwner(suite.fooService.Id).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(svc.Id))

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
		secondService := resourcetest.Resource(pbcatalog.ServiceType, "second-service").
			WithData(suite.T(), &pbcatalog.Service{}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(secondService.Id))

		secondEndpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "second-service").
			WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
				{
					Ports: map[string]*pbcatalog.WorkloadPort{
						"mesh": {
							Port:     20000,
							Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
						},
						"http": {
							Port:     8080,
							Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
						},
					},
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host:  "10.5.5.5",
							Ports: []string{"mesh", "http"},
						},
						{
							Host:  "10.6.6.6",
							Ports: []string{"mesh", "http"},
						},
					},
				},
			}}).
			WithOwner(secondService.Id).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(secondEndpoints.Id))

		// Update the endpoint references on the fooProxyStateTemplate.
		suite.fooEndpointRefs["test-cluster-2"] = &pbproxystate.EndpointRef{
			Id:        secondEndpoints.Id,
			MeshPort:  "mesh",
			RoutePort: "http",
		}

		oldVersion := suite.fooProxyStateTemplate.Version
		fooProxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "foo-pst").
			WithData(suite.T(), &pbmesh.ProxyStateTemplate{
				RequiredEndpoints:        suite.fooEndpointRefs,
				ProxyState:               &pbmesh.ProxyState{},
				RequiredLeafCertificates: suite.fooLeafRefs,
			}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(fooProxyStateTemplate.Id))

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

	})
}

// Sets up a full controller, and tests that reconciles are getting triggered for the leaf cert events it should.
// This test ensures when a CA is updated, the controller is triggered to update the leaf cert when it changes.
func (suite *xdsControllerTestSuite) TestController_ComputeAddUpdateDeleteLeafReferences() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Run the controller manager.
		mgr := controller.NewManager(suite.client, suite.runtime.Logger)
		mgr.Register(Controller(suite.mapper, suite.updater, suite.fetcher, suite.leafCertManager, suite.leafMapper, suite.leafCancels, "dc1"))
		mgr.SetRaftLeader(true)
		go mgr.Run(suite.ctx)

		suite.setupFooProxyStateTemplateWithReferences(tenancy)
		leafCertRef := suite.fooLeafRefs["foo-workload-identity"]
		fooLeafResRef := leafResourceRef(leafCertRef.Name, leafCertRef.Namespace, leafCertRef.Partition)

		// oldLeaf will store the original leaf from before we trigger a CA update.
		var oldLeaf *x509.Certificate

		// Assert that the expected ProxyState matches the actual ProxyState that PushChange was called with. This needs to
		// be in a retry block unlike the Reconcile tests because the controller triggers asynchronously.
		retry.Run(suite.T(), func(r *retry.R) {
			actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
			actualLeafs := suite.updater.GetLeafs(suite.fooProxyStateTemplate.Id.Name)
			// Assert on the status.
			suite.client.RequireStatusCondition(r, suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())
			// Assert that the endpoints computed in the controller matches the expected endpoints.
			prototest.AssertDeepEqual(r, suite.expectedFooProxyStateEndpoints, actualEndpoints)
			// Assert that the leafs computed in the controller matches the expected leafs.
			require.Len(r, actualLeafs, 1)
			for k, l := range actualLeafs {
				pemDecode, _ := pem.Decode([]byte(l.Cert))
				cert, err := x509.ParseCertificate(pemDecode.Bytes)
				oldLeaf = cert
				require.NoError(r, err)
				require.Equal(r, cert.URIs[0].String(), suite.expectedFooProxyStateSpiffes[k])
				// Check the state of the cancel functions map.
				_, ok := suite.leafCancels.Get(keyFromReference(fooLeafResRef))
				require.True(r, ok)
			}
		})

		// Update the CA, and ensure the leaf cert is different from the leaf certificate from the step above.
		suite.signer.UpdateCA(suite.T(), nil)
		retry.Run(suite.T(), func(r *retry.R) {
			actualLeafs := suite.updater.GetLeafs(suite.fooProxyStateTemplate.Id.Name)
			require.Len(r, actualLeafs, 1)
			for k, l := range actualLeafs {
				pemDecode, _ := pem.Decode([]byte(l.Cert))
				cert, err := x509.ParseCertificate(pemDecode.Bytes)
				// Ensure the leaf was actually updated by checking that the leaf we just got is different from the old leaf.
				require.NotEqual(r, oldLeaf.Raw, cert.Raw)
				require.NoError(r, err)
				require.Equal(r, suite.expectedFooProxyStateSpiffes[k], cert.URIs[0].String())
				// Check the state of the cancel functions map. Even though we've updated the leaf cert, we should still
				// have a watch going for it.
				_, ok := suite.leafCancels.Get(keyFromReference(fooLeafResRef))
				require.True(r, ok)
			}
		})

		// Delete the leaf references on the fooProxyStateTemplate
		delete(suite.fooLeafRefs, "foo-workload-identity")
		oldVersion := suite.fooProxyStateTemplate.Version
		fooProxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "foo-pst").
			WithData(suite.T(), &pbmesh.ProxyStateTemplate{
				RequiredEndpoints:        suite.fooEndpointRefs,
				ProxyState:               &pbmesh.ProxyState{},
				RequiredLeafCertificates: suite.fooLeafRefs,
			}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(fooProxyStateTemplate.Id))

		retry.Run(suite.T(), func(r *retry.R) {
			suite.client.RequireVersionChanged(r, fooProxyStateTemplate.Id, oldVersion)
		})

		// Ensure the leaf certificate watches were cancelled since we deleted the leaf reference.
		retry.Run(suite.T(), func(r *retry.R) {
			_, ok := suite.leafCancels.Get(keyFromReference(fooLeafResRef))
			require.False(r, ok)
		})
	})
}

// Sets up a full controller, and tests that reconciles are getting triggered for the leaf cert events it should.
// This test ensures that when a ProxyStateTemplate is deleted, the leaf watches are cancelled.
func (suite *xdsControllerTestSuite) TestController_ComputeLeafReferencesDeletePST() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Run the controller manager.
		mgr := controller.NewManager(suite.client, suite.runtime.Logger)
		mgr.Register(Controller(suite.mapper, suite.updater, suite.fetcher, suite.leafCertManager, suite.leafMapper, suite.leafCancels, "dc1"))
		mgr.SetRaftLeader(true)
		go mgr.Run(suite.ctx)

		suite.setupFooProxyStateTemplateWithReferences(tenancy)
		leafCertRef := suite.fooLeafRefs["foo-workload-identity"]
		fooLeafResRef := leafResourceRef(leafCertRef.Name, leafCertRef.Namespace, leafCertRef.Partition)

		// Assert that the expected ProxyState matches the actual ProxyState that PushChange was called with. This needs to
		// be in a retry block unlike the Reconcile tests because the controller triggers asynchronously.
		retry.Run(suite.T(), func(r *retry.R) {
			actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
			actualLeafs := suite.updater.GetLeafs(suite.fooProxyStateTemplate.Id.Name)
			// Assert on the status.
			suite.client.RequireStatusCondition(r, suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())
			// Assert that the endpoints computed in the controller matches the expected endpoints.
			prototest.AssertDeepEqual(r, suite.expectedFooProxyStateEndpoints, actualEndpoints)
			// Assert that the leafs computed in the controller matches the expected leafs.
			require.Len(r, actualLeafs, 1)
			for k, l := range actualLeafs {
				pemDecode, _ := pem.Decode([]byte(l.Cert))
				cert, err := x509.ParseCertificate(pemDecode.Bytes)
				require.NoError(r, err)
				require.Equal(r, cert.URIs[0].String(), suite.expectedFooProxyStateSpiffes[k])
				// Check the state of the cancel functions map.
				_, ok := suite.leafCancels.Get(keyFromReference(fooLeafResRef))
				require.True(r, ok)
			}
		})

		// Delete the fooProxyStateTemplate

		req := &pbresource.DeleteRequest{
			Id: suite.fooProxyStateTemplate.Id,
		}
		_, err := suite.client.Delete(suite.ctx, req)
		require.NoError(suite.T(), err)

		// Ensure the leaf certificate watches were cancelled since we deleted the leaf reference.
		retry.Run(suite.T(), func(r *retry.R) {
			_, ok := suite.leafCancels.Get(keyFromReference(fooLeafResRef))
			require.False(r, ok)
		})
	})
}

// Sets up a full controller, and tests that reconciles are getting triggered for the events it should.
func (suite *xdsControllerTestSuite) TestController_ComputeEndpointForProxyConnections() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Run the controller manager.
		mgr := controller.NewManager(suite.client, suite.runtime.Logger)

		mgr.Register(Controller(suite.mapper, suite.updater, suite.fetcher, suite.leafCertManager, suite.leafMapper, suite.leafCancels, "dc1"))
		mgr.SetRaftLeader(true)
		go mgr.Run(suite.ctx)

		// Set up fooEndpoints and fooProxyStateTemplate with a reference to fooEndpoints. These need to be stored
		// because the controller reconcile looks them up.
		suite.setupFooProxyStateTemplateWithReferences(tenancy)

		// Assert that the expected ProxyState matches the actual ProxyState that PushChange was called with. This needs to
		// be in a retry block unlike the Reconcile tests because the controller triggers asynchronously.
		retry.Run(suite.T(), func(r *retry.R) {
			actualEndpoints := suite.updater.GetEndpoints(suite.fooProxyStateTemplate.Id.Name)
			// Assert on the status.
			suite.client.RequireStatusCondition(r, suite.fooProxyStateTemplate.Id, ControllerName, status.ConditionAccepted())
			// Assert that the endpoints computed in the controller matches the expected endpoints.
			prototest.AssertDeepEqual(r, suite.expectedFooProxyStateEndpoints, actualEndpoints)
		})

		eventChannel := suite.updater.EventChannel()
		eventChannel <- controller.Event{Obj: &proxytracker.ProxyConnection{ProxyID: suite.fooProxyStateTemplate.Id}}

		// Wait for the proxy state template to be re-evaluated.
		proxyStateTemp := suite.client.WaitForNewVersion(suite.T(), suite.fooProxyStateTemplate.Id, suite.fooProxyStateTemplate.Version)
		require.NotNil(suite.T(), proxyStateTemp)
	})
}

// Setup: fooProxyStateTemplate with:
//   - an EndpointsRef to fooEndpoints
//   - a LeafCertificateRef to "foo-workload-identity"
//
// Saves all related resources to the suite so they can be looked up by the controller or modified if needed.
func (suite *xdsControllerTestSuite) setupFooProxyStateTemplateWithReferences(tenancy *pbresource.Tenancy) {
	fooService := resourcetest.Resource(pbcatalog.ServiceType, "foo-service").
		WithData(suite.T(), &pbcatalog.Service{}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooService.Id))

	fooEndpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "foo-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
					"http": {
						Port:     8080,
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.1.1.1",
						Ports: []string{"mesh", "http"},
					},
					{
						Host:  "10.2.2.2",
						Ports: []string{"mesh", "http"},
					},
				},
			},
		}}).
		WithTenancy(tenancy).
		WithOwner(fooService.Id).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooEndpoints.Id))

	fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id:        fooEndpoints.Id,
		MeshPort:  "mesh",
		RoutePort: "http",
	}

	fooRequiredLeafs := make(map[string]*pbproxystate.LeafCertificateRef)
	fooRequiredLeafs["foo-workload-identity"] = &pbproxystate.LeafCertificateRef{
		Name: "foo-workload-identity",
	}

	fooProxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "foo-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			RequiredEndpoints:        fooRequiredEndpoints,
			RequiredLeafCertificates: fooRequiredLeafs,
			ProxyState:               &pbmesh.ProxyState{},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooProxyStateTemplate.Id))

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, fooProxyStateTemplate.Id)
	})

	expectedFooLeafSpiffes := map[string]string{
		"foo-workload-identity": "spiffe://11111111-2222-3333-4444-555555555555.consul/ap/default/ns/default/identity/foo-workload-identity",
	}
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
		resource.DefaultPeerName: {
			TrustDomain: "some-trust-domain",
			Roots:       []string{"some-root", "some-other-root"},
		},
	}

	suite.fooService = fooService
	suite.fooEndpoints = fooEndpoints
	suite.fooEndpointRefs = fooRequiredEndpoints
	suite.fooLeafRefs = fooRequiredLeafs
	suite.fooProxyStateTemplate = fooProxyStateTemplate
	suite.expectedFooProxyStateEndpoints = expectedFooProxyStateEndpoints
	suite.expectedTrustBundle = expectedTrustBundle
	suite.expectedFooProxyStateSpiffes = expectedFooLeafSpiffes

	suite.T().Cleanup(func() {
		suite.fooEndpointRefs = make(map[string]*pbproxystate.EndpointRef)
		suite.fooLeafRefs = make(map[string]*pbproxystate.LeafCertificateRef)
		suite.expectedFooProxyStateEndpoints = make(map[string]*pbproxystate.Endpoints)
		suite.expectedTrustBundle = make(map[string]*pbproxystate.TrustBundle)
		suite.expectedFooProxyStateSpiffes = make(map[string]string)
	})
}

// Setup:
//   - fooProxyStateTemplate with an EndpointsRef to fooEndpoints and fooBarEndpoints.
//   - barProxyStateTemplate with an EndpointsRef to fooBarEndpoints.
//
// Saves all related resources to the suite so they can be modified if needed.
func (suite *xdsControllerTestSuite) setupFooBarProxyStateTemplateAndEndpoints(tenancy *pbresource.Tenancy) {
	fooService := resourcetest.Resource(pbcatalog.ServiceType, "foo-service").
		WithData(suite.T(), &pbcatalog.Service{}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooService.Id))

	fooEndpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "foo-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
					"http": {
						Port:     8080,
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.1.1.1",
						Ports: []string{"mesh", "http"},
					},
					{
						Host:  "10.2.2.2",
						Ports: []string{"mesh", "http"},
					},
				},
			},
		}}).
		WithOwner(fooService.Id).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooEndpoints.Id))

	fooBarService := resourcetest.Resource(pbcatalog.ServiceType, "foo-bar-service").
		WithData(suite.T(), &pbcatalog.Service{}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooBarService.Id))

	fooBarEndpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "foo-bar-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
			{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"mesh": {
						Port:     20000,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
					"http": {
						Port:     8080,
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "10.3.3.3",
						Ports: []string{"mesh", "http"},
					},
					{
						Host:  "10.4.4.4",
						Ports: []string{"mesh", "http"},
					},
				},
			},
		}}).
		WithOwner(fooBarService.Id).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooBarEndpoints.Id))

	fooRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	fooRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id:        fooEndpoints.Id,
		MeshPort:  "mesh",
		RoutePort: "http",
	}
	fooRequiredEndpoints["test-cluster-2"] = &pbproxystate.EndpointRef{
		Id:        fooBarEndpoints.Id,
		MeshPort:  "mesh",
		RoutePort: "http",
	}

	barRequiredEndpoints := make(map[string]*pbproxystate.EndpointRef)
	barRequiredEndpoints["test-cluster-1"] = &pbproxystate.EndpointRef{
		Id:        fooBarEndpoints.Id,
		MeshPort:  "mesh",
		RoutePort: "http",
	}

	fooProxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "foo-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			// Contains the foo and foobar endpoints.
			RequiredEndpoints: fooRequiredEndpoints,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(fooProxyStateTemplate.Id))

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, fooProxyStateTemplate.Id)
	})

	barProxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "bar-pst").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{
			// Contains the foobar endpoint.
			RequiredEndpoints: barRequiredEndpoints,
			ProxyState:        &pbmesh.ProxyState{},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(barProxyStateTemplate.Id))

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

	suite.T().Cleanup(func() {
		suite.barEndpointRefs = make(map[string]*pbproxystate.EndpointRef)
		suite.fooEndpointRefs = make(map[string]*pbproxystate.EndpointRef)
		suite.expectedFooProxyStateEndpoints = make(map[string]*pbproxystate.Endpoints)
		suite.expectedBarProxyStateEndpoints = make(map[string]*pbproxystate.Endpoints)
	})
}

func (suite *xdsControllerTestSuite) TestReconcile_prevWatchesToCancel() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		makeRef := func(names ...string) []*pbresource.Reference {
			out := make([]*pbresource.Reference, len(names))
			for i, name := range names {
				out[i] = &pbresource.Reference{
					Name: name,
					Type: &pbresource.Type{
						Group:        "g",
						GroupVersion: "v",
						Kind:         "k",
					},
					Tenancy: tenancy,
				}
			}
			return out
		}
		convert := func(input []*pbresource.Reference) []resource.ReferenceOrID {
			asInterface := make([]resource.ReferenceOrID, len(input))
			for i := range input {
				asInterface[i] = input[i]
			}
			return asInterface
		}

		cases := []struct {
			old    []*pbresource.Reference
			new    []*pbresource.Reference
			expect []*pbresource.Reference
		}{
			{
				old:    makeRef("a", "b", "c"),
				new:    makeRef("a", "c"),
				expect: makeRef("b"),
			},
			{
				old:    makeRef("a", "b", "c"),
				new:    makeRef("a", "b", "c"),
				expect: makeRef(),
			},
			{
				old:    makeRef(),
				new:    makeRef("a", "b"),
				expect: makeRef(),
			},
			{
				old:    makeRef("a", "b"),
				new:    makeRef(),
				expect: makeRef("a", "b"),
			},
			{
				old:    makeRef(),
				new:    makeRef(),
				expect: makeRef(),
			},
		}

		for _, tc := range cases {
			toCancel := prevWatchesToCancel(tc.old, convert(tc.new))
			require.ElementsMatch(suite.T(), toCancel, tc.expect)
		}
	})
}

func TestXdsController(t *testing.T) {
	suite.Run(t, new(xdsControllerTestSuite))
}

// TestReconcile_SidecarProxyGoldenFileInputs tests the Reconcile() by using
// the golden test output/expected files from the sidecar proxy tests as inputs
// to the XDS controller reconciliation.
// XDS controller reconciles the full ProxyStateTemplate object.  The fields
// that things that it focuses on are leaf certs, endpoints, and trust bundles,
// which is just a subset of the ProxyStateTemplate struct.  Prior to XDS controller
// reconciliation, the sidecar proxy controller will have reconciled the other parts
// of the ProxyStateTemplate.
// Since the XDS controller does act on the ProxyStateTemplate, the tests
// utilize that entire object rather than just the parts that XDS controller
// internals reconciles.  Namely, by using checking the full ProxyStateTemplate
// rather than just endpoints, leaf certs, and trust bundles, the test also ensures
// side effects or change in scope to XDS controller are not introduce mistakenly.
func (suite *xdsControllerTestSuite) TestReconcile_SidecarProxyGoldenFileInputs() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		path := "../sidecarproxy/builder/testdata"
		cases := []string{
			// destinations - please add in alphabetical order
			"destination/l4-single-destination-ip-port-bind-address",
			"destination/l4-single-destination-unix-socket-bind-address",
			"destination/l4-single-implicit-destination-tproxy",
			"destination/l4-multi-destination",
			"destination/l4-multiple-implicit-destinations-tproxy",
			"destination/l4-implicit-and-explicit-destinations-tproxy",
			"destination/mixed-multi-destination",
			"destination/multiport-l4-and-l7-multiple-implicit-destinations-tproxy",
			"destination/multiport-l4-and-l7-single-implicit-destination-tproxy",
			"destination/multiport-l4-and-l7-single-implicit-destination-with-multiple-workloads-tproxy",

			//sources - please add in alphabetical order
			"source/l7-expose-paths",
			"source/local-and-inbound-connections",
			"source/multiple-workload-addresses-with-specific-ports",
			"source/multiple-workload-addresses-without-ports",
			"source/multiport-l4-multiple-workload-addresses-with-specific-ports",
			"source/multiport-l4-multiple-workload-addresses-without-ports",
			"source/multiport-l4-workload-with-only-mesh-port",
			"source/multiport-l7-multiple-workload-addresses-with-specific-ports",
			"source/multiport-l7-multiple-workload-addresses-without-ports",
			"source/multiport-l7-multiple-workload-addresses-without-ports",
			"source/single-workload-address-without-ports",
		}

		for _, name := range cases {
			suite.Run(name, func() {
				// Create ProxyStateTemplate from the golden file.
				pst := JSONToProxyTemplate(suite.T(),
					golden.GetBytesAtFilePath(suite.T(), fmt.Sprintf("%s/%s-%s-%s.golden", path, name, tenancy.Partition, tenancy.Namespace)))

				// Destinations will need endpoint refs set up.
				if strings.Split(name, "/")[0] == "destination" && len(pst.ProxyState.Endpoints) == 0 {
					suite.addRequiredEndpointsAndRefs(pst, tenancy)
				}

				// Store the initial ProxyStateTemplate.
				proxyStateTemplate := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "test").
					WithData(suite.T(), pst).
					WithTenancy(tenancy).
					Write(suite.T(), suite.client)

				suite.T().Cleanup(suite.deleteResourceFunc(proxyStateTemplate.Id))

				// Check with resource service that it exists.
				retry.Run(suite.T(), func(r *retry.R) {
					suite.client.RequireResourceExists(r, proxyStateTemplate.Id)
				})

				// Track it in the mapper.
				suite.mapper.TrackItem(proxyStateTemplate.Id, []resource.ReferenceOrID{})

				// Run the reconcile, and since no ProxyStateTemplate is stored, this simulates a deletion.
				err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
					ID: proxyStateTemplate.Id,
				})
				require.NoError(suite.T(), err)
				require.NotNil(suite.T(), proxyStateTemplate)

				// Get the reconciled proxyStateTemplate to check the reconcile results.
				reconciledPS := suite.updater.Get(proxyStateTemplate.Id.Name)

				// Verify leaf cert contents then hard code them for comparison
				// and downstream tests since they change from test run to test run.
				require.NotEmpty(suite.T(), reconciledPS.LeafCertificates)
				reconciledPS.LeafCertificates = map[string]*pbproxystate.LeafCertificate{
					"test-identity": {
						Cert: "-----BEGIN CERTIFICATE-----\nMIICDjCCAbWgAwIBAgIBAjAKBggqhkjOPQQDAjAUMRIwEAYDVQQDEwlUZXN0IENB\nIDEwHhcNMjMxMDE2MTYxMzI5WhcNMjMxMDE2MTYyMzI5WjAAMFkwEwYHKoZIzj0C\nAQYIKoZIzj0DAQcDQgAErErAIosDPheZQGbxFQ4hYC/e9Fi4MG9z/zjfCnCq/oK9\nta/bGT+5orZqTmdN/ICsKQDhykxZ2u/Xr6845zhcJaOCAQowggEGMA4GA1UdDwEB\n/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwEwDAYDVR0TAQH/\nBAIwADApBgNVHQ4EIgQg3ogXVz9cqaK2B6xdiJYMa5NtT0KkYv7BA2dR7h9EcwUw\nKwYDVR0jBCQwIoAgq+C1mPlPoGa4lt7sSft1goN5qPGyBIB/3mUHJZKSFY8wbwYD\nVR0RAQH/BGUwY4Zhc3BpZmZlOi8vMTExMTExMTEtMjIyMi0zMzMzLTQ0NDQtNTU1\nNTU1NTU1NTU1LmNvbnN1bC9hcC9kZWZhdWx0L25zL2RlZmF1bHQvaWRlbnRpdHkv\ndGVzdC1pZGVudGl0eTAKBggqhkjOPQQDAgNHADBEAiB6L+t5bzRrBPhiQYNeA7fF\nUCuLWrdjW4Xbv3SLg0IKMgIgfRC5hEx+DqzQxTCP4sexX3hVWMjKoWmHdwiUcg+K\n/IE=\n-----END CERTIFICATE-----\n",
						Key:  "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIFIFkTIL1iUV4O/RpveVHzHs7ZzhSkvYIzbdXDttz9EooAoGCCqGSM49\nAwEHoUQDQgAErErAIosDPheZQGbxFQ4hYC/e9Fi4MG9z/zjfCnCq/oK9ta/bGT+5\norZqTmdN/ICsKQDhykxZ2u/Xr6845zhcJQ==\n-----END EC PRIVATE KEY-----\n",
					},
				}

				// Compare actual vs expected.
				actual := prototest.ProtoToJSON(suite.T(), reconciledPS)
				expected := golden.Get(suite.T(), actual, name+"-"+tenancy.Partition+"-"+tenancy.Namespace+".golden")
				require.JSONEq(suite.T(), expected, actual)
			})
		}
	})
}

func (suite *xdsControllerTestSuite) addRequiredEndpointsAndRefs(pst *pbmesh.ProxyStateTemplate, tenancy *pbresource.Tenancy) {
	//get service data
	serviceData := &pbcatalog.Service{}
	var vp uint32 = 7000
	requiredEps := make(map[string]*pbproxystate.EndpointRef)

	// iterate through clusters and set up endpoints for cluster/mesh port.
	for clusterName := range pst.ProxyState.Clusters {
		if clusterName == "null_route_cluster" || clusterName == "original-destination" {
			continue
		}

		//increment the random port number.
		vp++
		clusterNameSplit := strings.Split(clusterName, ".")
		port := clusterNameSplit[0]
		svcName := clusterNameSplit[1]

		// set up service data with port info.
		serviceData.Ports = append(serviceData.Ports, &pbcatalog.ServicePort{
			TargetPort:  port,
			VirtualPort: vp,
			Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
		})

		// create service.
		svc := resourcetest.Resource(pbcatalog.ServiceType, svcName).
			WithData(suite.T(), &pbcatalog.Service{}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(svc.Id))

		// create endpoints with svc as owner.
		eps := resourcetest.Resource(pbcatalog.ServiceEndpointsType, svcName).
			WithData(suite.T(), &pbcatalog.ServiceEndpoints{Endpoints: []*pbcatalog.Endpoint{
				{
					Ports: map[string]*pbcatalog.WorkloadPort{
						"mesh": {
							Port:     20000,
							Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
						},
						"http": {
							Port:     8080,
							Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
						},
					},
					Addresses: []*pbcatalog.WorkloadAddress{
						{
							Host:  "10.1.1.1",
							Ports: []string{"mesh", "http"},
						},
					},
				},
			}}).
			WithOwner(svc.Id).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		suite.T().Cleanup(suite.deleteResourceFunc(eps.Id))

		// add to working list of required endpoints.
		requiredEps[clusterName] = &pbproxystate.EndpointRef{
			Id:        eps.Id,
			MeshPort:  "mesh",
			RoutePort: "http",
		}
	}

	// set working list of required endpoints as proxy state's RequiredEndpoints.
	pst.RequiredEndpoints = requiredEps
}

func JSONToProxyTemplate(t *testing.T, json []byte) *pbmesh.ProxyStateTemplate {
	t.Helper()
	proxyTemplate := &pbmesh.ProxyStateTemplate{}
	m := protojson.UnmarshalOptions{}
	err := m.Unmarshal(json, proxyTemplate)
	require.NoError(t, err)
	return proxyTemplate
}

func (suite *xdsControllerTestSuite) runTestCaseWithTenancies(testCase func(tenancy *pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			testCase(tenancy)
		})
	}
}

func (suite *xdsControllerTestSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *xdsControllerTestSuite) deleteResourceFunc(id *pbresource.ID) func() {
	return func() {
		suite.client.MustDelete(suite.T(), id)
	}
}
