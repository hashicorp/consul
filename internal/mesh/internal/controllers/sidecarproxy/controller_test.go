// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/sidecarproxymapper"
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

type meshControllerTestSuite struct {
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
}

func (suite *meshControllerTestSuite) SetupTest() {
	resourceClient := svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes, auth.RegisterTypes)
	suite.client = resourcetest.NewClient(resourceClient)
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.ctx = testutil.TestContext(suite.T())

	suite.ctl = &reconciler{
		destinationsCache: sidecarproxycache.NewDestinationsCache(),
		proxyCfgCache:     sidecarproxycache.NewProxyConfigurationCache(),
		identitiesCache:   sidecarproxycache.NewIdentitiesCache(),
		getTrustDomain: func() (string, error) {
			return "test.consul", nil
		},
	}

	{
		// DB will be a service with a single workload, IN the mesh that will
		// be a destination of web.

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
		suite.dbWorkloadID = resourcetest.Resource(pbcatalog.WorkloadType, "db-abc").
			WithData(suite.T(), suite.dbWorkload).
			Write(suite.T(), resourceClient).Id

		suite.dbService = resourcetest.Resource(pbcatalog.ServiceType, "db-service").
			WithData(suite.T(), &pbcatalog.Service{
				Workloads:  &pbcatalog.WorkloadSelector{Names: []string{"db-abc"}},
				VirtualIps: []string{"1.1.1.1"},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				}}).
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
			Write(suite.T(), suite.client)

	}

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

	suite.apiWorkloadID = resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
		WithData(suite.T(), suite.apiWorkload).
		Write(suite.T(), resourceClient).Id

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

	suite.apiComputedTrafficPermissions = resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, suite.apiWorkload.Identity).
		WithData(suite.T(), suite.apiComputedTrafficPermissionsData).
		Write(suite.T(), resourceClient)

	suite.apiService = resourcetest.Resource(pbcatalog.ServiceType, "api-service").
		WithData(suite.T(), suite.apiServiceData).
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
		Write(suite.T(), suite.client.ResourceServiceClient)

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
	suite.webWorkload = resourcetest.Resource(pbcatalog.WorkloadType, "web-def").
		WithData(suite.T(), webWorkloadData).
		Write(suite.T(), suite.client)

	resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, webWorkloadData.Identity).
		WithData(suite.T(), &pbauth.ComputedTrafficPermissions{IsDefault: true}).
		Write(suite.T(), resourceClient)

	resourcetest.Resource(pbcatalog.ServiceType, "web").
		WithData(suite.T(), &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Names: []string{"web-def"}},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			}}).
		Write(suite.T(), suite.client)

	resourcetest.Resource(pbcatalog.ServiceEndpointsType, "web").
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
	}

	suite.proxyStateTemplate = builder.New(suite.apiWorkloadID, identityRef, "test.consul", "dc1", false, nil).
		BuildLocalApp(suite.apiWorkload, suite.apiComputedTrafficPermissionsData).
		Build()
}

func (suite *meshControllerTestSuite) TestReconcile_NoWorkload() {
	// This test ensures that removed workloads are ignored and don't result
	// in the creation of the proxy state template.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(pbmesh.ProxyStateTemplateType, "not-found"),
	})
	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, "not-found"))
}

func (suite *meshControllerTestSuite) TestReconcile_NonMeshWorkload() {
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
		Write(suite.T(), suite.client.ResourceServiceClient)

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(pbmesh.ProxyStateTemplateType, "test-non-mesh-api-workload"),
	})

	require.NoError(suite.T(), err)
	suite.client.RequireResourceNotFound(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, "test-non-mesh-api-workload"))
}

func (suite *meshControllerTestSuite) TestReconcile_NoExistingProxyStateTemplate() {
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(pbmesh.ProxyStateTemplateType, suite.apiWorkloadID.Name),
	})
	require.NoError(suite.T(), err)

	res := suite.client.RequireResourceExists(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, suite.apiWorkloadID.Name))
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res.Data)
	prototest.AssertDeepEqual(suite.T(), suite.apiWorkloadID, res.Owner)
}

func (suite *meshControllerTestSuite) TestReconcile_ExistingProxyStateTemplate_WithUpdates() {
	// This test ensures that we write a new proxy state template when there are changes.

	// Write the original.
	resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-abc").
		WithData(suite.T(), suite.proxyStateTemplate).
		WithOwner(suite.apiWorkloadID).
		Write(suite.T(), suite.client.ResourceServiceClient)

	// Update the apiWorkload.
	suite.apiWorkload.Ports["mesh"].Port = 21000
	updatedWorkloadID := resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
		WithData(suite.T(), suite.apiWorkload).
		Write(suite.T(), suite.client.ResourceServiceClient).Id

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(pbmesh.ProxyStateTemplateType, updatedWorkloadID.Name),
	})
	require.NoError(suite.T(), err)

	res := suite.client.RequireResourceExists(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, updatedWorkloadID.Name))
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res.Data)
	prototest.AssertDeepEqual(suite.T(), updatedWorkloadID, res.Owner)

	var updatedProxyStateTemplate pbmesh.ProxyStateTemplate
	err = res.Data.UnmarshalTo(&updatedProxyStateTemplate)
	require.NoError(suite.T(), err)

	// Check that our value is updated in the proxy state template.
	inboundListenerPort := updatedProxyStateTemplate.ProxyState.Listeners[0].
		BindAddress.(*pbproxystate.Listener_HostPort).HostPort.Port
	require.Equal(suite.T(), uint32(21000), inboundListenerPort)
}

func (suite *meshControllerTestSuite) TestReconcile_ExistingProxyStateTemplate_NoUpdates() {
	// This test ensures that we skip writing of the proxy state template when there are no changes to it.

	// Write the original.
	originalProxyState := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-abc").
		WithData(suite.T(), suite.proxyStateTemplate).
		WithOwner(suite.apiWorkloadID).
		Write(suite.T(), suite.client.ResourceServiceClient)

	// Update the metadata on the apiWorkload which should result in no changes.
	updatedWorkloadID := resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
		WithData(suite.T(), suite.apiWorkload).
		WithMeta("some", "meta").
		Write(suite.T(), suite.client.ResourceServiceClient).Id

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(pbmesh.ProxyStateTemplateType, updatedWorkloadID.Name),
	})
	require.NoError(suite.T(), err)

	updatedProxyState := suite.client.RequireResourceExists(suite.T(), resourceID(pbmesh.ProxyStateTemplateType, suite.apiWorkloadID.Name))
	resourcetest.RequireVersionUnchanged(suite.T(), updatedProxyState, originalProxyState.Version)
}

func (suite *meshControllerTestSuite) TestController() {
	// This is a comprehensive test that checks the overall controller behavior as various resources change state.
	// This should test interactions between the reconciler, the mappers, and the destinationsCache to ensure they work
	// together and produce expected result.

	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)

	// Initialize controller dependencies.
	var (
		destinationsCache   = sidecarproxycache.NewDestinationsCache()
		proxyCfgCache       = sidecarproxycache.NewProxyConfigurationCache()
		computedRoutesCache = sidecarproxycache.NewComputedRoutesCache()
		identitiesCache     = sidecarproxycache.NewIdentitiesCache()
		m                   = sidecarproxymapper.New(destinationsCache, proxyCfgCache, computedRoutesCache, identitiesCache)
	)
	trustDomainFetcher := func() (string, error) { return "test.consul", nil }

	mgr.Register(Controller(destinationsCache, proxyCfgCache, computedRoutesCache, identitiesCache, m, trustDomainFetcher, "dc1", false))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	var (
		// Create proxy state template IDs to check against in this test.
		apiProxyStateTemplateID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-abc").ID()
		webProxyStateTemplateID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "web-def").ID()

		apiComputedRoutesID = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.apiService.Id)
		dbComputedRoutesID  = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.dbService.Id)

		apiProxyStateTemplate *pbresource.Resource
		webProxyStateTemplate *pbresource.Resource
		webDestinations       *pbresource.Resource
	)

	testutil.RunStep(suite.T(), "proxy state template generation", func(t *testing.T) {
		// Check that proxy state template resource is generated for both the api and web workloads.
		retry.Run(t, func(r *retry.R) {
			suite.client.RequireResourceExists(r, apiProxyStateTemplateID)
			webProxyStateTemplate = suite.client.RequireResourceExists(r, webProxyStateTemplateID)
			apiProxyStateTemplate = suite.client.RequireResourceExists(r, apiProxyStateTemplateID)
		})
	})

	testutil.RunStep(suite.T(), "add explicit destinations and check that new proxy state is generated", func(t *testing.T) {
		// Write a default ComputedRoutes for api.
		routestest.ReconcileComputedRoutes(suite.T(), suite.client, apiComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](t, suite.apiService),
		)

		// Add a source service and check that a new proxy state is generated.
		webDestinations = resourcetest.Resource(pbmesh.DestinationsType, "web-destinations").
			WithData(suite.T(), &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{Names: []string{"web-def"}},
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

		webProxyStateTemplate = suite.client.WaitForNewVersion(t, webProxyStateTemplateID, webProxyStateTemplate.Version)

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
			WithData(suite.T(), &pbcatalog.Workload{
				Identity:  "api-identity",
				Addresses: suite.apiWorkload.Addresses,
				Ports:     nonMeshPorts}).
			Write(suite.T(), suite.client)

		suite.apiService = resourcetest.ResourceID(suite.apiService.Id).
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

		// Check status on the pbmesh.Destinations resource.
		serviceRef := resource.ReferenceToString(resource.Reference(suite.apiService.Id, ""))
		suite.client.WaitForStatusCondition(t, webDestinations.Id, ControllerName,
			status.ConditionMeshProtocolNotFound(serviceRef))

		// We should get a new web proxy template resource because this destination should be removed.
		webProxyStateTemplate = suite.client.WaitForNewVersion(t, webProxyStateTemplateID, webProxyStateTemplate.Version)

		requireExplicitDestinationsNotFound(t, "api", webProxyStateTemplate)
	})

	testutil.RunStep(suite.T(), "update ports to be mesh again", func(t *testing.T) {
		// Update destination's service endpoints back to mesh and check that we get a new web proxy resource re-generated
		// and that the status on Upstreams resource is updated to be empty.
		suite.runtime.Logger.Trace("updating ports to mesh")

		resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
			WithData(suite.T(), suite.apiWorkload).
			Write(suite.T(), suite.client)

		suite.apiService = resourcetest.Resource(pbcatalog.ServiceType, "api-service").
			WithData(suite.T(), suite.apiServiceData).
			Write(suite.T(), suite.client.ResourceServiceClient)

		resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-service").
			WithData(suite.T(), suite.apiEndpointsData).
			Write(suite.T(), suite.client.ResourceServiceClient)

		// Refresh the computed routes in light of api losing a mesh port.
		routestest.ReconcileComputedRoutes(suite.T(), suite.client, apiComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](t, suite.apiService),
		)

		serviceRef := resource.ReferenceToString(resource.Reference(suite.apiService.Id, ""))
		suite.client.WaitForStatusCondition(t, webDestinations.Id, ControllerName,
			status.ConditionMeshProtocolFound(serviceRef))

		// We should also get a new web proxy template resource as this destination should be added again.
		webProxyStateTemplate = suite.client.WaitForNewVersion(t, webProxyStateTemplateID, webProxyStateTemplate.Version)

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
		_, err := suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: webDestinations.Id})
		require.NoError(t, err)

		webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

		// Write a default ComputedRoutes for db, so it's eligible.
		dbCR := routestest.ReconcileComputedRoutes(suite.T(), suite.client, dbComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](t, suite.dbService),
		)
		require.NotNil(t, dbCR)

		// Enable transparent proxy for the web proxy.
		resourcetest.Resource(pbmesh.ProxyConfigurationType, "proxy-config").
			WithData(t, &pbmesh.ProxyConfiguration{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"web"},
				},
				DynamicConfig: &pbmesh.DynamicConfig{
					Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
					TransparentProxy: &pbmesh.TransparentProxy{
						OutboundListenerPort: 15001,
					},
				},
			}).Write(suite.T(), suite.client)

		webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)
		apiProxyStateTemplate = suite.client.WaitForNewVersion(t, apiProxyStateTemplateID, apiProxyStateTemplate.Version)

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

		apiProxyStateTemplate = suite.client.WaitForNewVersion(t, apiProxyStateTemplateID, apiProxyStateTemplate.Version)

		suite.runtime.Logger.Trace("creating computed traffic permissions")
		resourcetest.Resource(pbauth.ComputedTrafficPermissionsType, suite.apiWorkload.Identity).
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
			WithTenancy(resource.DefaultNamespacedTenancy()).
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

		webProxyStateTemplate = suite.client.WaitForNewVersion(t, webProxyStateTemplateID, webProxyStateTemplate.Version)

		requireImplicitDestinationsFound(t, "api", webProxyStateTemplate)
		requireImplicitDestinationsFound(t, "db", webProxyStateTemplate)
	})
}

func (suite *meshControllerTestSuite) TestControllerDefaultAllow() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)

	// Initialize controller dependencies.
	var (
		destinationsCache   = sidecarproxycache.NewDestinationsCache()
		proxyCfgCache       = sidecarproxycache.NewProxyConfigurationCache()
		computedRoutesCache = sidecarproxycache.NewComputedRoutesCache()
		identitiesCache     = sidecarproxycache.NewIdentitiesCache()
		m                   = sidecarproxymapper.New(destinationsCache, proxyCfgCache, computedRoutesCache, identitiesCache)
	)
	trustDomainFetcher := func() (string, error) { return "test.consul", nil }

	mgr.Register(Controller(destinationsCache, proxyCfgCache, computedRoutesCache, identitiesCache, m, trustDomainFetcher, "dc1", true))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	var (
		// Create proxy state template IDs to check against in this test.
		apiProxyStateTemplateID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api-abc").ID()
		webProxyStateTemplateID = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "web-def").ID()
	)

	retry.Run(suite.T(), func(r *retry.R) {
		webProxyStateTemplate := suite.client.RequireResourceExists(r, webProxyStateTemplateID)
		apiProxyStateTemplate := suite.client.RequireResourceExists(r, apiProxyStateTemplateID)

		// Default deny because api has non-empty computed traffic permissions.
		assertTrafficPermissionDefaultPolicy(r, false, apiProxyStateTemplate)
		assertTrafficPermissionDefaultPolicy(r, true, webProxyStateTemplate)
	})
}

func TestMeshController(t *testing.T) {
	suite.Run(t, new(meshControllerTestSuite))
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
					routerName := x.L7.Name
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

func resourceID(rtype *pbresource.Type, name string) *pbresource.ID {
	return &pbresource.ID{
		Type: rtype,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		},
		Name: name,
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
