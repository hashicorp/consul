// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"context"
	"fmt"
	mockres "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type controllerTestSuite struct {
	suite.Suite

	client  *resourcetest.Client
	runtime controller.Runtime

	ctl *reconciler
	ctx context.Context

	apiWorkloadID                     *pbresource.ID
	apiWorkload                       *pbcatalog.Workload
	apiComputedTrafficPermissions     *pbresource.Resource
	apiComputedTrafficPermissionsData *pbauth.ComputedTrafficPermissions
	apiService                        *pbresource.Resource
	apiServiceData                    *pbcatalog.Service
	apiEndpoints                      *pbresource.Resource
	apiEndpointsData                  *pbcatalog.ServiceEndpoints
	webWorkload                       *pbresource.Resource

	dbWorkloadID    *pbresource.ID
	dbWorkload      *pbcatalog.Workload
	dbService       *pbresource.Resource
	dbEndpoints     *pbresource.Resource
	dbEndpointsData *pbcatalog.ServiceEndpoints

	proxyStateTemplate *pbmesh.ProxyStateTemplate
	tenancies          []*pbresource.Tenancy
}

func (suite *controllerTestSuite) SetupTest() {
	suite.tenancies = resourcetest.TestTenancies()
	mockTenancyBridge := &mockres.MockTenancyBridge{}
	for _, tenancy := range suite.tenancies {
		mockTenancyBridge.On("PartitionExists", tenancy.Partition).Return(true, nil)
		mockTenancyBridge.On("NamespaceExists", tenancy.Partition, tenancy.Namespace).Return(true, nil)
		mockTenancyBridge.On("IsPartitionMarkedForDeletion", tenancy.Partition).Return(false, nil)
		mockTenancyBridge.On("IsNamespaceMarkedForDeletion", tenancy.Partition, tenancy.Namespace).Return(false, nil)
	}
	cfg := mockres.Config{
		TenancyBridge: mockTenancyBridge,
	}
	resourceClient := svctest.RunResourceServiceWithConfig(suite.T(), cfg, types.Register, catalog.RegisterTypes, auth.RegisterTypes)
	suite.client = resourcetest.NewClient(resourceClient)
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.ctx = testutil.TestContext(suite.T())

	suite.ctl = &reconciler{
		cache: cache.New(),
		getTrustDomain: func() (string, error) {
			return "test.consul", nil
		},
	}

	suite.dbWorkload = &pbcatalog.Workload{
		Identity: "db-identity",
		Addresses: []*pbcatalog.WorkloadAddress{
			{Host: "10.0.4.1"},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}

	suite.apiServiceData = &pbcatalog.Service{
		Workloads:  &pbcatalog.WorkloadSelector{Names: []string{"api-abc"}},
		VirtualIps: []string{"1.1.1.1"},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}

	suite.apiComputedTrafficPermissionsData = &pbauth.ComputedTrafficPermissions{
		IsDefault: false,
		AllowPermissions: []*pbauth.Permission{
			{
				Sources: []*pbauth.Source{
					{
						IdentityName: "foo",
						Namespace:    "default",
						Partition:    "default",
						Peer:         "local",
					},
				},
			},
		},
	}
}

func (suite *controllerTestSuite) setupSuiteWithTenancy(tenancy *pbresource.Tenancy) {

	suite.apiWorkload = &pbcatalog.Workload{
		Identity: "api-identity",
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "10.0.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}

	webWorkloadData := &pbcatalog.Workload{
		Identity: "web-identity",
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "10.0.0.2",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"tcp":  {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}
	// DB will be a service with a single workload, IN the mesh that will
	// be a destination of web.

	suite.dbWorkloadID = resourcetest.Resource(pbcatalog.WorkloadType, "db-abc").
		WithData(suite.T(), suite.dbWorkload).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client.ResourceServiceClient).Id

	suite.dbService = resourcetest.Resource(pbcatalog.ServiceType, "db-service").
		WithData(suite.T(), &pbcatalog.Service{
			Workloads:  &pbcatalog.WorkloadSelector{Names: []string{"db-abc"}},
			VirtualIps: []string{"1.1.1.1"},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			}}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.dbEndpointsData = &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: suite.dbWorkloadID,
				Addresses: suite.dbWorkload.Addresses,
				Ports:     suite.dbWorkload.Ports,
				Identity:  "db-identity",
			},
		},
	}

	suite.dbEndpoints = resourcetest.Resource(pbcatalog.ServiceEndpointsType, "db-service").
		WithData(suite.T(), suite.dbEndpointsData).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.apiWorkloadID = resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
		WithTenancy(tenancy).
		WithData(suite.T(), suite.apiWorkload).
		Write(suite.T(), suite.client.ResourceServiceClient).Id

	suite.apiComputedTrafficPermissions = resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, suite.apiWorkload.Identity).
		WithData(suite.T(), suite.apiComputedTrafficPermissionsData).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client.ResourceServiceClient)

	suite.apiService = resourcetest.Resource(pbcatalog.ServiceType, "api-service").
		WithData(suite.T(), suite.apiServiceData).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client.ResourceServiceClient)

	suite.apiEndpointsData = &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: suite.apiWorkloadID,
				Addresses: suite.apiWorkload.Addresses,
				Ports:     suite.apiWorkload.Ports,
				Identity:  "api-identity",
			},
		},
	}

	suite.apiEndpoints = resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-service").
		WithData(suite.T(), suite.apiEndpointsData).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client.ResourceServiceClient)

	suite.webWorkload = resourcetest.Resource(pbcatalog.WorkloadType, "web-def").
		WithData(suite.T(), webWorkloadData).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, webWorkloadData.Identity).
		WithData(suite.T(), &pbauth.ComputedTrafficPermissions{IsDefault: true}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client.ResourceServiceClient)

	resourcetest.Resource(pbcatalog.ServiceType, "web").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Names: []string{"web-def"}},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			}}).
		Write(suite.T(), suite.client)

	resourcetest.Resource(pbcatalog.ServiceEndpointsType, "web").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: suite.webWorkload.Id,
					Addresses: webWorkloadData.Addresses,
					Ports:     webWorkloadData.Ports,
					Identity:  "web-identity",
				},
			},
		}).Write(suite.T(), suite.client)

	identityRef := &pbresource.Reference{
		Name:    suite.apiWorkload.Identity,
		Tenancy: suite.apiWorkloadID.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}

	suite.proxyStateTemplate = builder.New(resource.ReplaceType(pbmesh.ProxyStateTemplateType, suite.apiWorkloadID),
		identityRef, "test.consul", "dc1", false, nil).
		BuildLocalApp(suite.apiWorkload, suite.apiComputedTrafficPermissionsData).
		Build()
}

func (suite *controllerTestSuite) TestWorkloadPortProtocolsFromService_NoServicesInCache() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dataFetcher := fetcher.New(suite.client, suite.ctl.cache)

		workload := resourcetest.Resource(pbcatalog.WorkloadType, "api-workload").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
			}).
			Build()

		decWorkload := resourcetest.MustDecode[*pbcatalog.Workload](suite.T(), workload)
		workloadPorts, err := suite.ctl.workloadPortProtocolsFromService(suite.ctx, dataFetcher, decWorkload, suite.runtime.Logger)
		require.NoError(suite.T(), err)
		prototest.AssertDeepEqual(suite.T(), pbcatalog.Protocol_PROTOCOL_TCP, workloadPorts["tcp"].GetProtocol())
	})
}

func (suite *controllerTestSuite) TestWorkloadPortProtocolsFromService_ServiceNotFound() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		c := cache.New()
		dataFetcher := fetcher.New(suite.client, c)
		ctrl := &reconciler{
			cache: c,
			getTrustDomain: func() (string, error) {
				return "test.consul", nil
			},
		}
		svc := resourcetest.Resource(pbcatalog.ServiceType, "not-found").
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"api-workload"},
				},
			}).
			WithTenancy(tenancy).
			Build()

		decSvc := resourcetest.MustDecode[*pbcatalog.Service](suite.T(), svc)
		c.TrackService(decSvc)

		workload := resourcetest.Resource(pbcatalog.WorkloadType, "api-workload").
			WithData(suite.T(), &pbcatalog.Workload{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
			}).
			WithTenancy(tenancy).
			Build()

		decWorkload := resourcetest.MustDecode[*pbcatalog.Workload](suite.T(), workload)

		workloadPorts, err := ctrl.workloadPortProtocolsFromService(suite.ctx, dataFetcher, decWorkload, suite.runtime.Logger)
		require.NoError(suite.T(), err)
		prototest.AssertDeepEqual(suite.T(), pbcatalog.Protocol_PROTOCOL_TCP, workloadPorts["tcp"].GetProtocol())
		// Check that the service is no longer in cache.
		require.Nil(suite.T(), c.ServicesForWorkload(workload.Id))
	})
}

func (suite *controllerTestSuite) TestWorkloadPortProtocolsFromService() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		c := cache.New()
		dataFetcher := fetcher.New(suite.client, c)
		ctrl := &reconciler{
			cache: c,
			getTrustDomain: func() (string, error) {
				return "test.consul", nil
			},
		}
		svc1 := resourcetest.Resource(pbcatalog.ServiceType, "api-1").
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"api-workload"},
				},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "http1",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "conflict",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
			}).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		decSvc := resourcetest.MustDecode[*pbcatalog.Service](suite.T(), svc1)
		c.TrackService(decSvc)

		svc2 := resourcetest.Resource(pbcatalog.ServiceType, "api-2").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"api-workload"},
				},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "http2",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP2,
					},
					{
						TargetPort: "conflict",
						Protocol:   pbcatalog.Protocol_PROTOCOL_GRPC,
					},
				},
			}).
			Write(suite.T(), suite.client)

		decSvc = resourcetest.MustDecode[*pbcatalog.Service](suite.T(), svc2)
		c.TrackService(decSvc)

		workload := resourcetest.Resource(pbcatalog.WorkloadType, "api-workload").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Ports: map[string]*pbcatalog.WorkloadPort{
					"http1":              {Port: 8080},
					"http2":              {Port: 9090},
					"conflict":           {Port: 9091},
					"not-selected":       {Port: 8081},
					"specified-protocol": {Port: 8082, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
					"mesh":               {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			}).
			WithTenancy(tenancy).
			Build()

		decWorkload := resourcetest.MustDecode[*pbcatalog.Workload](suite.T(), workload)

		expWorkloadPorts := map[string]*pbcatalog.WorkloadPort{
			// This protocol should be inherited from service 1.
			"http1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},

			// this protocol should be inherited from service 2.
			"http2": {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},

			// This port is not selected by the service and should default to tcp.
			"not-selected": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},

			// This port has conflicting protocols in each service and so it should default to tcp.
			"conflict": {Port: 9091, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},

			// These port should keep its existing protocol.
			"specified-protocol": {Port: 8082, Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			"mesh":               {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		}

		workloadPorts, err := ctrl.workloadPortProtocolsFromService(suite.ctx, dataFetcher, decWorkload, suite.runtime.Logger)
		require.NoError(suite.T(), err)

		prototest.AssertDeepEqual(suite.T(), expWorkloadPorts, workloadPorts)
	})
}

func (suite *controllerTestSuite) TestReconcile_NoWorkload() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// This test ensures that removed workloads are ignored and don't result
		// in the creation of the proxy state template.
		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: resourceID(pbmesh.ProxyStateTemplateType, "not-found", tenancy),
		})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, "not-found", tenancy))
	})
}

func (suite *controllerTestSuite) TestReconcile_NonMeshWorkload() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// This test ensures that non-mesh workloads are ignored by the controller.

		nonMeshWorkload := &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host: "10.0.0.1",
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			},
		}

		resourcetest.Resource(pbcatalog.WorkloadType, "test-non-mesh-api-workload").
			WithData(suite.T(), nonMeshWorkload).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client.ResourceServiceClient)

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: resourceID(pbmesh.ProxyStateTemplateType, "test-non-mesh-api-workload", tenancy),
		})

		require.NoError(suite.T(), err)
		suite.client.RequireResourceNotFound(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, "test-non-mesh-api-workload", tenancy))
	})
}

func (suite *controllerTestSuite) TestReconcile_NoExistingProxyStateTemplate() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: resourceID(pbmesh.ProxyStateTemplateType, suite.apiWorkloadID.Name, tenancy),
		})
		require.NoError(suite.T(), err)

		res := suite.client.RequireResourceExists(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, suite.apiWorkloadID.Name, tenancy))
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), res.Data)
		prototest.AssertDeepEqual(suite.T(), suite.apiWorkloadID, res.Owner)
	})
}

func (suite *controllerTestSuite) TestReconcile_ExistingProxyStateTemplate_WithUpdates() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// This test ensures that we write a new proxy state template when there are changes.

		// Write the original.
		resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-abc").
			WithData(suite.T(), suite.proxyStateTemplate).
			WithOwner(suite.apiWorkloadID).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client.ResourceServiceClient)

		// Update the apiWorkload and check that we default the port to tcp if it's unspecified.
		suite.apiWorkload.Ports["tcp"].Protocol = pbcatalog.Protocol_PROTOCOL_UNSPECIFIED

		updatedWorkloadID := resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.apiWorkload).
			Write(suite.T(), suite.client.ResourceServiceClient).Id

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: resourceID(pbmesh.ProxyStateTemplateType, updatedWorkloadID.Name, tenancy),
		})
		require.NoError(suite.T(), err)

		res := suite.client.RequireResourceExists(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, updatedWorkloadID.Name, tenancy))
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), res.Data)
		prototest.AssertDeepEqual(suite.T(), updatedWorkloadID, res.Owner)

		var updatedProxyStateTemplate pbmesh.ProxyStateTemplate
		err = res.Data.UnmarshalTo(&updatedProxyStateTemplate)
		require.NoError(suite.T(), err)

		// Check that our value is updated in the proxy state template.
		require.Len(suite.T(), updatedProxyStateTemplate.ProxyState.Listeners, 1)
		require.Len(suite.T(), updatedProxyStateTemplate.ProxyState.Listeners[0].Routers, 1)

		l4InboundRouter := updatedProxyStateTemplate.ProxyState.Listeners[0].
			Routers[0].GetL4()
		require.NotNil(suite.T(), l4InboundRouter)
	})
}

func (suite *controllerTestSuite) TestReconcile_ExistingProxyStateTemplate_NoUpdates() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// This test ensures that we skip writing of the proxy state template when there are no changes to it.

		// Write the original.
		originalProxyState := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-abc").
			WithData(suite.T(), suite.proxyStateTemplate).
			WithOwner(suite.apiWorkloadID).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client.ResourceServiceClient)

		// Update the metadata on the apiWorkload which should result in no changes.
		updatedWorkloadID := resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
			WithData(suite.T(), suite.apiWorkload).
			WithMeta("some", "meta").
			Write(suite.T(), suite.client.ResourceServiceClient).Id

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: resourceID(pbmesh.ProxyStateTemplateType, updatedWorkloadID.Name, tenancy),
		})
		require.NoError(suite.T(), err)

		updatedProxyState := suite.client.RequireResourceExists(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, suite.apiWorkloadID.Name, tenancy))
		resourcetest.RequireVersionUnchanged(suite.T(), updatedProxyState, originalProxyState.Version)
	})
}

func (suite *controllerTestSuite) TestController() {
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)

	// Initialize controller dependencies.
	c := cache.New()
	trustDomainFetcher := func() (string, error) { return "test.consul", nil }

	mgr.Register(Controller(c, trustDomainFetcher, "dc1", false))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// This is a comprehensive test that checks the overall controller behavior as various resources change state.
		// This should test interactions between the reconciler, the mappers, and the destinationsCache to ensure they work
		// together and produce expected result.

		// Run the controller manager
		var (
			// Create proxy state template IDs to check against in this test.
			apiProxyStateTemplateID = resourcetest.ResourceWithTenancy(pbmesh.ProxyStateTemplateType, "api-abc", tenancy).ID()
			webProxyStateTemplateID = resourcetest.ResourceWithTenancy(pbmesh.ProxyStateTemplateType, "web-def", tenancy).ID()

			apiComputedRoutesID = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.apiService.Id)
			dbComputedRoutesID  = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.dbService.Id)

			apiProxyStateTemplate   *pbresource.Resource
			webProxyStateTemplate   *pbresource.Resource
			webComputedDestinations *pbresource.Resource
		)

		testutil.RunStep(suite.T(), "proxy state template generation", func(t *testing.T) {
			// Check that proxy state template resource is generated for both the api and web workloads.
			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceExists(r, apiProxyStateTemplateID)
				webProxyStateTemplate = suite.client.RequireResourceExists(r, webProxyStateTemplateID)
				apiProxyStateTemplate = suite.client.RequireResourceExists(r, apiProxyStateTemplateID)
			})
		})

		// Write a default ComputedRoutes for api.
		routestest.ReconcileComputedRoutes(suite.T(), suite.client, apiComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.apiService),
		)

		// Add a source service and check that a new proxy state is generated.
		webComputedDestinations = resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webWorkload.Id.Name).
			WithTenancy(tenancy).
			WithData(suite.T(), &pbmesh.ComputedExplicitDestinations{
				Destinations: []*pbmesh.Destination{
					{
						DestinationRef:  resource.Reference(suite.apiService.Id, ""),
						DestinationPort: "tcp",
						ListenAddr: &pbmesh.Destination_IpPort{
							IpPort: &pbmesh.IPPortAddress{
								Ip:   "127.0.0.1",
								Port: 1234,
							},
						},
					},
				},
			}).Write(suite.T(), suite.client)

		testutil.RunStep(suite.T(), "add explicit destinations and check that new proxy state is generated", func(t *testing.T) {
			webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

			requireExplicitDestinationsFound(t, "api", webProxyStateTemplate)
		})

		testutil.RunStep(suite.T(), "update api's ports to be non-mesh", func(t *testing.T) {
			// Update destination's service endpoints and workload to be non-mesh
			// and check that:
			// * api's proxy state template is deleted
			// * we get a new web proxy resource re-generated
			// * the status on Upstreams resource is updated with a validation error
			nonMeshPorts := map[string]*pbcatalog.WorkloadPort{
				"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			}

			// Note: the order matters here because in reality service endpoints will only
			// be reconciled after the workload has been updated, and so we need to write the
			// workload and service before we write service endpoints.
			resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
				WithTenancy(tenancy).
				WithData(suite.T(), &pbcatalog.Workload{
					Identity:  "api-identity",
					Addresses: suite.apiWorkload.Addresses,
					Ports:     nonMeshPorts}).
				Write(suite.T(), suite.client)

			suite.apiService = resourcetest.ResourceID(suite.apiService.Id).
				WithTenancy(tenancy).
				WithData(t, &pbcatalog.Service{
					Workloads:  &pbcatalog.WorkloadSelector{Names: []string{"api-abc"}},
					VirtualIps: []string{"1.1.1.1"},
					Ports: []*pbcatalog.ServicePort{
						{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
						// {TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					},
				}).
				Write(suite.T(), suite.client)

			resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-service").
				WithTenancy(tenancy).
				WithData(suite.T(), &pbcatalog.ServiceEndpoints{
					Endpoints: []*pbcatalog.Endpoint{
						{
							TargetRef: suite.apiWorkloadID,
							Addresses: suite.apiWorkload.Addresses,
							Ports:     nonMeshPorts,
							Identity:  "api-identity",
						},
					},
				}).
				Write(suite.T(), suite.client.ResourceServiceClient)

			// Refresh the computed routes in light of api losing a mesh port.
			routestest.ReconcileComputedRoutes(suite.T(), suite.client, apiComputedRoutesID,
				resourcetest.MustDecode[*pbcatalog.Service](t, suite.apiService),
			)

			// Check that api proxy template is gone.
			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceNotFound(r, apiProxyStateTemplateID)
			})

			// We should get a new web proxy template resource because this destination should be removed.
			webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

			requireExplicitDestinationsNotFound(t, "api", webProxyStateTemplate)
		})

		testutil.RunStep(suite.T(), "update ports to be mesh again", func(t *testing.T) {
			// Update destination's service endpoints back to mesh and check that we get a new web proxy resource re-generated
			// and that the status on Upstreams resource is updated to be empty.
			suite.runtime.Logger.Trace("updating ports to mesh")

			resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
				WithTenancy(tenancy).
				WithData(suite.T(), suite.apiWorkload).
				Write(suite.T(), suite.client)

			suite.apiService = resourcetest.Resource(pbcatalog.ServiceType, "api-service").
				WithData(suite.T(), suite.apiServiceData).
				WithTenancy(tenancy).
				Write(suite.T(), suite.client.ResourceServiceClient)

			resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-service").
				WithTenancy(tenancy).
				WithData(suite.T(), suite.apiEndpointsData).
				Write(suite.T(), suite.client.ResourceServiceClient)

			// Refresh the computed routes in light of api losing a mesh port.
			routestest.ReconcileComputedRoutes(suite.T(), suite.client, apiComputedRoutesID,
				resourcetest.MustDecode[*pbcatalog.Service](t, suite.apiService),
			)

			// We should also get a new web proxy template resource as this destination should be added again.
			webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

			requireExplicitDestinationsFound(t, "api", webProxyStateTemplate)
		})

		testutil.RunStep(suite.T(), "delete the proxy state template and check re-generation", func(t *testing.T) {
			// Delete the proxy state template resource and check that it gets regenerated.
			suite.runtime.Logger.Trace("deleting web proxy")
			_, err := suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: webProxyStateTemplateID})
			require.NoError(suite.T(), err)

			webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)
			requireExplicitDestinationsFound(t, "api", webProxyStateTemplate)
		})

		testutil.RunStep(suite.T(), "add implicit upstream and enable tproxy", func(t *testing.T) {
			// Delete explicit destinations resource.
			suite.runtime.Logger.Trace("deleting web destinations")
			_, err := suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: webComputedDestinations.Id})
			require.NoError(t, err)

			webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

			// Write a default ComputedRoutes for db, so it's eligible.
			dbCR := routestest.ReconcileComputedRoutes(suite.T(), suite.client, dbComputedRoutesID,
				resourcetest.MustDecode[*pbcatalog.Service](t, suite.dbService),
			)
			require.NotNil(t, dbCR)

			// Enable transparent proxy for the web proxy.
			resourcetest.Resource(pbmesh.ComputedProxyConfigurationType, suite.webWorkload.Id.Name).
				WithTenancy(tenancy).
				WithData(t, &pbmesh.ComputedProxyConfiguration{
					DynamicConfig: &pbmesh.DynamicConfig{
						Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
						TransparentProxy: &pbmesh.TransparentProxy{
							OutboundListenerPort: 15001,
						},
					},
				}).Write(suite.T(), suite.client)

			webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)
			apiProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), apiProxyStateTemplateID, apiProxyStateTemplate.Version)

			requireImplicitDestinationsFound(t, "api", webProxyStateTemplate)
			requireImplicitDestinationsFound(t, "db", webProxyStateTemplate)
		})

		testutil.RunStep(suite.T(), "traffic permissions", func(t *testing.T) {
			// Global default deny applies to all identities.
			assertTrafficPermissionDefaultPolicy(t, false, apiProxyStateTemplate)
			assertTrafficPermissionDefaultPolicy(t, false, webProxyStateTemplate)

			suite.runtime.Logger.Trace("deleting computed traffic permissions")
			_, err := suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: suite.apiComputedTrafficPermissions.Id})
			require.NoError(t, err)
			suite.client.WaitForDeletion(t, suite.apiComputedTrafficPermissions.Id)

			apiProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), apiProxyStateTemplateID, apiProxyStateTemplate.Version)

			suite.runtime.Logger.Trace("creating computed traffic permissions")
			resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, suite.apiWorkload.Identity).
				WithTenancy(tenancy).
				WithData(t, suite.apiComputedTrafficPermissionsData).
				Write(t, suite.client)

			suite.client.WaitForNewVersion(t, apiProxyStateTemplateID, apiProxyStateTemplate.Version)
		})

		testutil.RunStep(suite.T(), "add an HTTPRoute with a simple split on the tcp port", func(t *testing.T) {
			// NOTE: because at this point we have tproxy in all-to-all mode, we will get an
			// implicit upstream on 'db'

			// Create a route NOT in the state store, only to more easily feed
			// into the generator.
			routeData := &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{{
					Ref:  resource.Reference(suite.dbService.Id, ""),
					Port: "", // implicitly applies to 'http'
				}},
				Rules: []*pbmesh.HTTPRouteRule{{
					BackendRefs: []*pbmesh.HTTPBackendRef{
						{
							BackendRef: &pbmesh.BackendReference{
								Ref:  resource.Reference(suite.apiService.Id, ""),
								Port: "tcp",
							},
							Weight: 60,
						},
						{
							BackendRef: &pbmesh.BackendReference{
								Ref:  resource.Reference(suite.dbService.Id, ""),
								Port: "", // assumed to be 'http'
							},
							Weight: 40,
						},
					},
				}},
			}
			route := resourcetest.Resource(pbmesh.HTTPRouteType, "db-http-route").
				WithTenancy(tenancy).
				WithData(t, routeData).
				Build()
			require.NoError(t, types.MutateHTTPRoute(route))
			require.NoError(t, types.ValidateHTTPRoute(route))

			dbCRID := resource.ReplaceType(pbmesh.ComputedRoutesType, suite.dbService.Id)

			dbCR := routestest.ReconcileComputedRoutes(suite.T(), suite.client, dbCRID,
				resourcetest.MustDecode[*pbmesh.HTTPRoute](t, route),
				resourcetest.MustDecode[*pbcatalog.Service](t, suite.dbService),
				resourcetest.MustDecode[*pbcatalog.Service](t, suite.apiService),
			)
			require.NotNil(t, dbCR, "computed routes for db was deleted instead of created")

			webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

			requireImplicitDestinationsFound(t, "api", webProxyStateTemplate)
			requireImplicitDestinationsFound(t, "db", webProxyStateTemplate)
		})
	})
}

func (suite *controllerTestSuite) TestControllerDefaultAllow() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Run the controller manager
		mgr := controller.NewManager(suite.client, suite.runtime.Logger)

		// Initialize controller dependencies.
		c := cache.New()
		trustDomainFetcher := func() (string, error) { return "test.consul", nil }

		mgr.Register(Controller(c, trustDomainFetcher, "dc1", true))
		mgr.SetRaftLeader(true)
		go mgr.Run(suite.ctx)

		var (
			// Create proxy state template IDs to check against in this test.
			apiProxyStateTemplateID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-abc").WithTenancy(tenancy).ID()
			webProxyStateTemplateID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "web-def").WithTenancy(tenancy).ID()
		)

		retry.Run(suite.T(), func(r *retry.R) {
			webProxyStateTemplate := suite.client.RequireResourceExists(r, webProxyStateTemplateID)
			apiProxyStateTemplate := suite.client.RequireResourceExists(r, apiProxyStateTemplateID)

			// Default deny because api has non-empty computed traffic permissions.
			assertTrafficPermissionDefaultPolicy(r, false, apiProxyStateTemplate)
			assertTrafficPermissionDefaultPolicy(r, true, webProxyStateTemplate)
		})
	})
}

func TestMeshController(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func requireExplicitDestinationsFound(t *testing.T, name string, tmplResource *pbresource.Resource) {
	requireExplicitDestinations(t, name, tmplResource, true)
}

func requireExplicitDestinationsNotFound(t *testing.T, name string, tmplResource *pbresource.Resource) {
	requireExplicitDestinations(t, name, tmplResource, false)
}

func requireExplicitDestinations(t *testing.T, name string, tmplResource *pbresource.Resource, found bool) {
	t.Helper()

	var tmpl pbmesh.ProxyStateTemplate
	err := tmplResource.Data.UnmarshalTo(&tmpl)
	require.NoError(t, err)

	// Check outbound listener.
	var foundListener bool
	for _, l := range tmpl.ProxyState.Listeners {
		if strings.Contains(l.Name, name) && l.Direction == pbproxystate.Direction_DIRECTION_OUTBOUND {
			foundListener = true
			break
		}
	}

	require.Equal(t, found, foundListener)

	requireClustersAndEndpoints(t, name, &tmpl, found)
}

func requireImplicitDestinationsFound(t *testing.T, name string, tmplResource *pbresource.Resource) {
	t.Helper()

	var tmpl pbmesh.ProxyStateTemplate
	err := tmplResource.Data.UnmarshalTo(&tmpl)
	require.NoError(t, err)

	// Check outbound listener.
	var foundListener bool
	for _, l := range tmpl.ProxyState.Listeners {
		if strings.Contains(l.Name, xdscommon.OutboundListenerName) && l.Direction == pbproxystate.Direction_DIRECTION_OUTBOUND {
			foundListener = true

			// Check the listener filter chain
			for _, r := range l.Routers {
				foundByName := false
				switch x := r.Destination.(type) {
				case *pbproxystate.Router_L4:
					// TcpProxy is easy, so just having this exist is
					// sufficient. We don't have to deep inspect it. We care
					// that there IS a listener matching the destination. If
					// there is a TCPRoute with a split or a rename we don't
					// care here.
					foundByName = true
				case *pbproxystate.Router_L7:
					require.NotNil(t, x.L7.Route)
					routerName := x.L7.Route.Name
					foundByName = strings.Contains(routerName, name)
				default:
					t.Fatalf("unexpected type of destination: %T", r.Destination)
				}

				if foundByName {
					// We expect that there is a filter chain match for transparent proxy destinations.
					require.NotNil(t, r.Match)
					require.NotEmpty(t, r.Match.PrefixRanges)
					break
				}
			}
			break
		}
	}
	require.True(t, foundListener)

	requireClustersAndEndpoints(t, name, &tmpl, true)
}

func requireClustersAndEndpoints(t *testing.T, name string, tmpl *pbmesh.ProxyStateTemplate, found bool) {
	t.Helper()

	var foundCluster bool
	for c := range tmpl.ProxyState.Clusters {
		if strings.Contains(c, name) {
			foundCluster = true
			break
		}
	}

	require.Equal(t, found, foundCluster)

	var foundEndpoints bool
	for c := range tmpl.RequiredEndpoints {
		if strings.Contains(c, name) {
			foundEndpoints = true
			break
		}
	}

	require.Equal(t, found, foundEndpoints)
}

func resourceID(rtype *pbresource.Type, name string, tenancy *pbresource.Tenancy) *pbresource.ID {
	return &pbresource.ID{
		Type:    rtype,
		Tenancy: tenancy,
		Name:    name,
	}
}

func assertTrafficPermissionDefaultPolicy(t resourcetest.T, defaultAllow bool, resource *pbresource.Resource) {
	dec := resourcetest.MustDecode[*pbmesh.ProxyStateTemplate](t, resource)
	var listener *pbproxystate.Listener
	for _, l := range dec.Data.ProxyState.Listeners {
		if l.Name == "public_listener" {
			listener = l
			break
		}
	}
	require.Len(t, listener.Routers, 1)
	l4 := listener.Routers[0].GetL4()
	require.NotNil(t, l4)
	require.Equal(t, defaultAllow, l4.TrafficPermissions.DefaultAllow)
}

func (suite *controllerTestSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *controllerTestSuite) cleanupResources() {

	suite.client.MustDelete(suite.T(), suite.apiWorkloadID)
	suite.client.MustDelete(suite.T(), suite.apiComputedTrafficPermissions.Id)
	suite.client.MustDelete(suite.T(), suite.apiService.Id)
	suite.client.MustDelete(suite.T(), suite.apiEndpoints.Id)
	suite.client.MustDelete(suite.T(), suite.webWorkload.Id)
	suite.client.MustDelete(suite.T(), suite.dbWorkloadID)
	suite.client.MustDelete(suite.T(), suite.dbService.Id)
	suite.client.MustDelete(suite.T(), suite.dbEndpoints.Id)
}

func (suite *controllerTestSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupSuiteWithTenancy(tenancy)
			suite.T().Cleanup(func() {
				suite.cleanupResources()
			})
			t(tenancy)
		})
	}
}
