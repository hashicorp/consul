// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	ctrlStatus "github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

type dataFetcherSuite struct {
	suite.Suite

	ctx    context.Context
	client pbresource.ResourceServiceClient
	rt     controller.Runtime

	api1Service                 *pbresource.Resource
	api1ServiceData             *pbcatalog.Service
	api2Service                 *pbresource.Resource
	api2ServiceData             *pbcatalog.Service
	api1ServiceEndpoints        *pbresource.Resource
	api1ServiceEndpointsData    *pbcatalog.ServiceEndpoints
	api2ServiceEndpoints        *pbresource.Resource
	api2ServiceEndpointsData    *pbcatalog.ServiceEndpoints
	webComputedDestinationsData *pbmesh.ComputedExplicitDestinations
	webProxy                    *pbresource.Resource
	webWorkload                 *pbresource.Resource
}

func (suite *dataFetcherSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.client = svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}

	suite.api1ServiceData = &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}
	suite.api1Service = resourcetest.Resource(pbcatalog.ServiceType, "api-1").
		WithData(suite.T(), suite.api1ServiceData).
		Write(suite.T(), suite.client)

	suite.api1ServiceEndpointsData = &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				Identity: "api-1-identity",
			},
		},
	}
	suite.api1ServiceEndpoints = resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-1").
		WithData(suite.T(), suite.api1ServiceEndpointsData).
		Write(suite.T(), suite.client)

	suite.api2ServiceData = &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp1", VirtualPort: 9080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "tcp2", VirtualPort: 9081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}
	suite.api2Service = resourcetest.Resource(pbcatalog.ServiceType, "api-2").
		WithData(suite.T(), suite.api2ServiceData).
		Write(suite.T(), suite.client)

	suite.api2ServiceEndpointsData = &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.2"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp1": {Port: 9080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"tcp2": {Port: 9081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				Identity: "api-2-identity",
			},
		},
	}
	suite.api2ServiceEndpoints = resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-2").
		WithData(suite.T(), suite.api2ServiceEndpointsData).
		Write(suite.T(), suite.client)

	suite.webComputedDestinationsData = &pbmesh.ComputedExplicitDestinations{
		Destinations: []*pbmesh.Destination{
			{
				DestinationRef:  resource.Reference(suite.api1Service.Id, ""),
				DestinationPort: "tcp",
			},
			{
				DestinationRef:  resource.Reference(suite.api2Service.Id, ""),
				DestinationPort: "tcp1",
			},
			{
				DestinationRef:  resource.Reference(suite.api2Service.Id, ""),
				DestinationPort: "tcp2",
			},
		},
	}

	suite.webProxy = resourcetest.Resource(pbmesh.ProxyStateTemplateType, "web-abc").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{}).
		Write(suite.T(), suite.client)

	suite.webWorkload = resourcetest.Resource(pbcatalog.WorkloadType, "web-abc").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.2"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(suite.T(), suite.client)
}

func (suite *dataFetcherSuite) TestFetcher_FetchWorkload_WorkloadNotFound() {
	identityID := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-abc").ID()

	// Create cache and pre-populate it.
	c := cache.New()

	f := Fetcher{
		cache:  c,
		client: suite.client,
	}

	workloadID := resourcetest.Resource(pbcatalog.WorkloadType, "not-found").ID()

	// Track workload with its identity.
	workload := resourcetest.Resource(pbcatalog.WorkloadType, workloadID.GetName()).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), &pbcatalog.Workload{
			Identity: identityID.Name,
		}).Build()

	c.TrackWorkload(resourcetest.MustDecode[*pbcatalog.Workload](suite.T(), workload))

	// Now fetch the workload so that we can check that it's been removed from cache.
	_, err := f.FetchWorkload(context.Background(), workloadID)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), c.WorkloadsByWorkloadIdentity(identityID))
}

func (suite *dataFetcherSuite) TestFetcher_FetchWorkload_WorkloadFound() {
	identityID := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-abc").ID()

	// Create cache and pre-populate it.
	c := cache.New()

	f := Fetcher{
		cache:  c,
		client: suite.client,
	}

	workload := resourcetest.Resource(pbcatalog.WorkloadType, "service-workload-abc").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), &pbcatalog.Workload{
			Identity: identityID.Name,
			Ports: map[string]*pbcatalog.WorkloadPort{
				"foo": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host:  "10.0.0.1",
					Ports: []string{"foo"},
				},
			},
		}).Write(suite.T(), suite.client)

	// This call should track the workload's identity
	_, err := f.FetchWorkload(context.Background(), workload.Id)
	require.NoError(suite.T(), err)

	// Check that the workload is tracked
	workload.Id.Uid = ""
	prototest.AssertElementsMatch(suite.T(), []*pbresource.ID{workload.Id}, c.WorkloadsByWorkloadIdentity(identityID))
}

func (suite *dataFetcherSuite) TestFetcher_FetchExplicitDestinationsData() {
	c := cache.New()

	var (
		api1ServiceRef = resource.Reference(suite.api1Service.Id, "")
	)

	f := Fetcher{
		cache:  c,
		client: suite.client,
	}

	testutil.RunStep(suite.T(), "computed destinations not found", func(t *testing.T) {
		// First add computed destination to cache so we can check if it's untracked later.
		compDest := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
			WithData(t, &pbmesh.ComputedExplicitDestinations{
				Destinations: []*pbmesh.Destination{
					{
						DestinationRef:  api1ServiceRef,
						DestinationPort: "tcp1",
					},
				},
			}).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			Build()
		c.TrackComputedDestinations(resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, compDest))

		// We will try to fetch explicit destinations for a proxy that doesn't have one.
		destinations, _, err := f.FetchExplicitDestinationsData(suite.ctx, suite.webProxy.Id)
		require.NoError(t, err)
		require.Nil(t, destinations)

		// Check that cache no longer has this destination.
		require.Nil(t, c.ComputedDestinationsByService(resource.IDFromReference(api1ServiceRef)))
	})

	testutil.RunStep(suite.T(), "invalid destinations: service not found", func(t *testing.T) {
		notFoundServiceRef := resourcetest.Resource(pbcatalog.ServiceType, "not-found").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			ReferenceNoSection()

		compDest := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
			WithData(t, &pbmesh.ComputedExplicitDestinations{
				Destinations: []*pbmesh.Destination{
					{
						DestinationRef:  notFoundServiceRef,
						DestinationPort: "tcp",
					},
				},
			}).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			Write(t, suite.client)

		destinations, status, err := f.FetchExplicitDestinationsData(suite.ctx, suite.webProxy.Id)
		require.NoError(t, err)
		require.Nil(t, destinations)
		cachedCompDestIDs := c.ComputedDestinationsByService(resource.IDFromReference(notFoundServiceRef))
		compDest.Id.Uid = ""
		prototest.AssertElementsMatch(t, []*pbresource.ID{compDest.Id}, cachedCompDestIDs)

		// Check that the status is created.
		prototest.AssertContainsElement(t, status.Conditions,
			ctrlStatus.ConditionDestinationServiceNotFound(resource.ReferenceToString(notFoundServiceRef)))

	})

	testutil.RunStep(suite.T(), "invalid destinations: service not on mesh", func(t *testing.T) {
		apiNonMeshServiceData := &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			},
		}
		resourcetest.ResourceID(suite.api1Service.Id).
			WithData(t, apiNonMeshServiceData).
			Write(t, suite.client)
		compDest := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
			WithData(t, &pbmesh.ComputedExplicitDestinations{
				Destinations: []*pbmesh.Destination{
					{
						DestinationRef:  api1ServiceRef,
						DestinationPort: "tcp",
					},
				},
			}).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			Write(t, suite.client)

		destinations, status, err := f.FetchExplicitDestinationsData(suite.ctx, suite.webProxy.Id)
		require.NoError(t, err)
		require.Nil(t, destinations)
		cachedCompDestIDs := c.ComputedDestinationsByService(resource.IDFromReference(api1ServiceRef))
		compDest.Id.Uid = ""
		prototest.AssertElementsMatch(t, []*pbresource.ID{compDest.Id}, cachedCompDestIDs)

		prototest.AssertContainsElement(t, status.Conditions,
			ctrlStatus.ConditionMeshProtocolNotFound(resource.ReferenceToString(api1ServiceRef)))
	})

	testutil.RunStep(suite.T(), "invalid destinations: destination port not found", func(t *testing.T) {
		resourcetest.ResourceID(suite.api1Service.Id).
			WithData(t, &pbcatalog.Service{
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "some-other-port", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			}).
			Write(t, suite.client)
		compDest := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
			WithData(t, &pbmesh.ComputedExplicitDestinations{
				Destinations: []*pbmesh.Destination{
					{
						DestinationRef:  api1ServiceRef,
						DestinationPort: "tcp",
					},
				},
			}).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			Write(t, suite.client)

		destinations, status, err := f.FetchExplicitDestinationsData(suite.ctx, suite.webProxy.Id)
		require.NoError(t, err)
		require.Nil(t, destinations)
		cachedCompDestIDs := c.ComputedDestinationsByService(resource.IDFromReference(api1ServiceRef))
		compDest.Id.Uid = ""
		prototest.AssertElementsMatch(t, []*pbresource.ID{compDest.Id}, cachedCompDestIDs)

		prototest.AssertContainsElement(t, status.Conditions,
			ctrlStatus.ConditionDestinationPortNotFound(resource.ReferenceToString(api1ServiceRef), "tcp"))
	})

	suite.api1Service = resourcetest.ResourceID(suite.api1Service.Id).
		WithData(suite.T(), suite.api1ServiceData).
		Write(suite.T(), suite.client)

	suite.api2Service = resourcetest.ResourceID(suite.api2Service.Id).
		WithData(suite.T(), suite.api2ServiceData).
		Write(suite.T(), suite.client)

	testutil.RunStep(suite.T(), "invalid destinations: destination is pointing to a mesh port", func(t *testing.T) {
		// Create a computed destinations resource pointing to the mesh port.
		compDest := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
			WithData(t, &pbmesh.ComputedExplicitDestinations{
				Destinations: []*pbmesh.Destination{
					{
						DestinationRef:  api1ServiceRef,
						DestinationPort: "mesh",
					},
				},
			}).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			Write(t, suite.client)

		destinations, status, err := f.FetchExplicitDestinationsData(suite.ctx, suite.webProxy.Id)
		require.NoError(t, err)
		require.Empty(t, destinations)

		cachedCompDestIDs := c.ComputedDestinationsByService(resource.IDFromReference(api1ServiceRef))
		compDest.Id.Uid = ""
		prototest.AssertElementsMatch(t, []*pbresource.ID{compDest.Id}, cachedCompDestIDs)

		serviceRef := resource.ReferenceToString(api1ServiceRef)
		prototest.AssertContainsElement(t, status.Conditions,
			ctrlStatus.ConditionMeshProtocolDestinationPort(serviceRef, "mesh"))
	})

	compDest := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
		WithData(suite.T(), suite.webComputedDestinationsData).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Write(suite.T(), suite.client)

	testutil.RunStep(suite.T(), "invalid destinations: destination is pointing to a port but computed routes is not aware of it yet", func(t *testing.T) {
		apiNonTCPServiceData := &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}
		apiNonTCPService := resourcetest.ResourceID(suite.api1Service.Id).
			WithData(t, apiNonTCPServiceData).
			Build()

		api1ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, suite.api1Service.Id)
		api1ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api1ComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), apiNonTCPService),
		)
		require.NotNil(suite.T(), api1ComputedRoutes)

		serviceRef := resource.ReferenceToString(api1ServiceRef)

		// This destination points to TCP, but the computed routes is stale and only knows about HTTP.
		destinations, status, err := f.FetchExplicitDestinationsData(suite.ctx, suite.webProxy.Id)
		require.NoError(t, err)

		// Check that the status is generated correctly.
		prototest.AssertContainsElement(t, status.Conditions,
			ctrlStatus.ConditionDestinationComputedRoutesPortNotFound(serviceRef, "tcp"))

		// Check that we didn't return any destinations.
		require.Nil(t, destinations)

		// Check that destination service is still in cache because it's still referenced from the pbmesh.Destinations
		// resource.
		cachedCompDestIDs := c.ComputedDestinationsByService(resource.IDFromReference(api1ServiceRef))
		compDest.Id.Uid = ""
		prototest.AssertElementsMatch(t, []*pbresource.ID{compDest.Id}, cachedCompDestIDs)
	})

	testutil.RunStep(suite.T(), "happy path", func(t *testing.T) {
		// Write a default ComputedRoutes for api1 and api2.
		var (
			api1ComputedRoutesID = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.api1Service.Id)
			api2ComputedRoutesID = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.api2Service.Id)
		)
		api1ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api1ComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api1Service),
		)
		require.NotNil(suite.T(), api1ComputedRoutes)
		api2ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api2ComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api2Service),
		)
		require.NotNil(suite.T(), api2ComputedRoutes)

		resourcetest.ResourceID(suite.api1Service.Id)

		expectedDestinations := []*intermediate.Destination{
			{
				Explicit: suite.webComputedDestinationsData.Destinations[0],
				Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api1Service),
				ComputedPortRoutes: routestest.MutateTargets(suite.T(), api1ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
					switch {
					case resource.ReferenceOrIDMatch(suite.api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
						se := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](suite.T(), suite.api1ServiceEndpoints)
						details.ServiceEndpointsId = se.Resource.Id
						details.ServiceEndpoints = se.Data
						details.IdentityRefs = []*pbresource.Reference{{
							Name:    "api-1-identity",
							Tenancy: suite.api1Service.Id.Tenancy,
						}}
					}
				}),
			},
			{
				Explicit: suite.webComputedDestinationsData.Destinations[1],
				Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api2Service),
				ComputedPortRoutes: routestest.MutateTargets(suite.T(), api2ComputedRoutes.Data, "tcp1", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
					switch {
					case resource.ReferenceOrIDMatch(suite.api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp1":
						se := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](suite.T(), suite.api2ServiceEndpoints)
						details.ServiceEndpointsId = se.Resource.Id
						details.ServiceEndpoints = se.Data
						details.IdentityRefs = []*pbresource.Reference{{
							Name:    "api-2-identity",
							Tenancy: suite.api2Service.Id.Tenancy,
						}}
					}
				}),
			},
			{
				Explicit: suite.webComputedDestinationsData.Destinations[2],
				Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api2Service),
				ComputedPortRoutes: routestest.MutateTargets(suite.T(), api2ComputedRoutes.Data, "tcp2", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
					switch {
					case resource.ReferenceOrIDMatch(suite.api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp2":
						se := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](suite.T(), suite.api2ServiceEndpoints)
						details.ServiceEndpointsId = se.Resource.Id
						details.ServiceEndpoints = se.Data
						details.IdentityRefs = []*pbresource.Reference{{
							Name:    "api-2-identity",
							Tenancy: suite.api2Service.Id.Tenancy,
						}}
					}
				}),
			},
		}
		expectedConditions := []*pbresource.Condition{ctrlStatus.ConditionAllDestinationsValid()}

		actualDestinations, status, err := f.FetchExplicitDestinationsData(suite.ctx, suite.webProxy.Id)
		require.NoError(t, err)

		// Check that all statuses have "happy" conditions.
		prototest.AssertElementsMatch(t, expectedConditions, status.Conditions)

		// Check that we've computed expanded destinations correctly.
		prototest.AssertElementsMatch(t, expectedDestinations, actualDestinations)
	})
}

func (suite *dataFetcherSuite) TestFetcher_FetchImplicitDestinationsData() {
	// Create a few other services to be implicit upstreams.
	api3Service := resourcetest.Resource(pbcatalog.ServiceType, "api-3").
		WithData(suite.T(), &pbcatalog.Service{
			VirtualIps: []string{"192.1.1.1"},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}).
		Write(suite.T(), suite.client)

	api3ServiceEndpointsData := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: &pbresource.ID{
					Name:    "api-3-abc",
					Tenancy: api3Service.Id.Tenancy,
					Type:    pbcatalog.WorkloadType,
				},
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				Identity: "api-3-identity",
			},
		},
	}
	api3ServiceEndpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, "api-3").
		WithData(suite.T(), api3ServiceEndpointsData).
		Write(suite.T(), suite.client)

	// Write a default ComputedRoutes for api1, api2, and api3.
	var (
		api1ComputedRoutesID = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.api1Service.Id)
		api2ComputedRoutesID = resource.ReplaceType(pbmesh.ComputedRoutesType, suite.api2Service.Id)
		api3ComputedRoutesID = resource.ReplaceType(pbmesh.ComputedRoutesType, api3Service.Id)
	)
	api1ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api1ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api1Service),
	)
	require.NotNil(suite.T(), api1ComputedRoutes)
	api2ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api2ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api2Service),
	)
	require.NotNil(suite.T(), api2ComputedRoutes)
	api3ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api3ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](suite.T(), api3Service),
	)
	require.NotNil(suite.T(), api3ComputedRoutes)

	existingDestinations := []*intermediate.Destination{
		{
			Explicit: suite.webComputedDestinationsData.Destinations[0],
			Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api1Service),
			ComputedPortRoutes: routestest.MutateTargets(suite.T(), api1ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
				switch {
				case resource.ReferenceOrIDMatch(suite.api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
					se := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](suite.T(), suite.api1ServiceEndpoints)
					details.ServiceEndpointsId = se.Resource.Id
					details.ServiceEndpoints = se.Data
					details.IdentityRefs = []*pbresource.Reference{{
						Name:    "api-1-identity",
						Tenancy: suite.api1Service.Id.Tenancy,
					}}
				}
			}),
		},
		{
			Explicit: suite.webComputedDestinationsData.Destinations[1],
			Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api2Service),
			ComputedPortRoutes: routestest.MutateTargets(suite.T(), api2ComputedRoutes.Data, "tcp1", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
				switch {
				case resource.ReferenceOrIDMatch(suite.api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp1":
					se := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](suite.T(), suite.api2ServiceEndpoints)
					details.ServiceEndpointsId = se.Resource.Id
					details.ServiceEndpoints = se.Data
					details.IdentityRefs = []*pbresource.Reference{{
						Name:    "api-2-identity",
						Tenancy: suite.api1Service.Id.Tenancy,
					}}
				}
			}),
		},
		{
			Explicit: suite.webComputedDestinationsData.Destinations[2],
			Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api2Service),
			ComputedPortRoutes: routestest.MutateTargets(suite.T(), api2ComputedRoutes.Data, "tcp2", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
				switch {
				case resource.ReferenceOrIDMatch(suite.api2Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp2":
					se := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](suite.T(), suite.api2ServiceEndpoints)
					details.ServiceEndpointsId = se.Resource.Id
					details.ServiceEndpoints = se.Data
					details.IdentityRefs = []*pbresource.Reference{{
						Name:    "api-2-identity",
						Tenancy: suite.api1Service.Id.Tenancy,
					}}
				}
			}),
		},
		{
			// implicit
			Service: resourcetest.MustDecode[*pbcatalog.Service](suite.T(), api3Service),
			ComputedPortRoutes: routestest.MutateTargets(suite.T(), api3ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
				switch {
				case resource.ReferenceOrIDMatch(api3Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
					se := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](suite.T(), api3ServiceEndpoints)
					details.ServiceEndpointsId = se.Resource.Id
					details.ServiceEndpoints = se.Data
					details.IdentityRefs = []*pbresource.Reference{{
						Name:    "api-3-identity",
						Tenancy: suite.api1Service.Id.Tenancy,
					}}
				}
			}),
			VirtualIPs: []string{"192.1.1.1"},
		},
	}

	f := Fetcher{
		client: suite.client,
	}

	actualDestinations, err := f.FetchImplicitDestinationsData(context.Background(), suite.webProxy.Id, existingDestinations)
	require.NoError(suite.T(), err)

	prototest.AssertElementsMatch(suite.T(), existingDestinations, actualDestinations)
}

func TestDataFetcher(t *testing.T) {
	suite.Run(t, new(dataFetcherSuite))
}
