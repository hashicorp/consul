// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	meshStatus "github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestIsMeshEnabled(t *testing.T) {
	cases := map[string]struct {
		ports map[string]*pbcatalog.WorkloadPort
		exp   bool
	}{
		"nil ports": {
			ports: nil,
			exp:   false,
		},
		"empty ports": {
			ports: make(map[string]*pbcatalog.WorkloadPort),
			exp:   false,
		},
		"no mesh ports": {
			ports: map[string]*pbcatalog.WorkloadPort{
				"p1": {Port: 1000, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				"p2": {Port: 2000, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			},
			exp: false,
		},
		"one mesh port": {
			ports: map[string]*pbcatalog.WorkloadPort{
				"p1": {Port: 1000, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				"p2": {Port: 2000, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				"p3": {Port: 3000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
			exp: true,
		},
		"multiple mesh ports": {
			ports: map[string]*pbcatalog.WorkloadPort{
				"p1": {Port: 1000, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				"p2": {Port: 2000, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				"p3": {Port: 3000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				"p4": {Port: 4000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
			exp: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.exp, IsMeshEnabled(c.ports))
		})
	}
}

type dataFetcherSuite struct {
	suite.Suite

	ctx    context.Context
	client pbresource.ResourceServiceClient
	rt     controller.Runtime

	api1Service              *pbresource.Resource
	api2Service              *pbresource.Resource
	api1ServiceEndpoints     *pbresource.Resource
	api1ServiceEndpointsData *pbcatalog.ServiceEndpoints
	api2ServiceEndpoints     *pbresource.Resource
	api2ServiceEndpointsData *pbcatalog.ServiceEndpoints
	webDestinations          *pbresource.Resource
	webDestinationsData      *pbmesh.Upstreams
	webProxy                 *pbresource.Resource
	webWorkload              *pbresource.Resource
}

func (suite *dataFetcherSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.client = svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}

	suite.api1Service = resourcetest.Resource(catalog.ServiceType, "api-1").
		WithData(suite.T(), &pbcatalog.Service{}).
		Write(suite.T(), suite.client)

	suite.api1ServiceEndpointsData = &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
				Identity: "api-1-identity",
			},
		},
	}
	suite.api1ServiceEndpoints = resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
		WithData(suite.T(), suite.api1ServiceEndpointsData).Write(suite.T(), suite.client)

	suite.api2Service = resourcetest.Resource(catalog.ServiceType, "api-2").
		WithData(suite.T(), &pbcatalog.Service{}).
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
	suite.api2ServiceEndpoints = resourcetest.Resource(catalog.ServiceEndpointsType, "api-2").
		WithData(suite.T(), suite.api2ServiceEndpointsData).Write(suite.T(), suite.client)

	suite.webDestinationsData = &pbmesh.Upstreams{
		Upstreams: []*pbmesh.Upstream{
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

	suite.webDestinations = resourcetest.Resource(types.UpstreamsType, "web-destinations").
		WithData(suite.T(), suite.webDestinationsData).
		Write(suite.T(), suite.client)

	suite.webProxy = resourcetest.Resource(types.ProxyStateTemplateType, "web-abc").
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{}).
		Write(suite.T(), suite.client)

	suite.webWorkload = resourcetest.Resource(catalog.WorkloadType, "web-abc").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.2"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
		}).
		Write(suite.T(), suite.client)
}

func (suite *dataFetcherSuite) TestFetcher_FetchWorkload_WorkloadNotFound() {
	// Test that when workload is not found, we remove it from cache.

	proxyID := resourcetest.Resource(types.ProxyStateTemplateType, "service-workload-abc").ID()

	// Create cache and pre-populate it.
	destCache := sidecarproxycache.NewDestinationsCache()
	proxyCfgCache := sidecarproxycache.NewProxyConfigurationCache()
	dest1 := intermediate.CombinedDestinationRef{
		ServiceRef:             resourcetest.Resource(catalog.ServiceType, "test-service-1").ReferenceNoSection(),
		Port:                   "tcp",
		ExplicitDestinationsID: resourcetest.Resource(types.UpstreamsType, "test-servicedestinations-1").ID(),
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(proxyID): {},
		},
	}
	dest2 := intermediate.CombinedDestinationRef{
		ServiceRef:             resourcetest.Resource(catalog.ServiceType, "test-service-2").ReferenceNoSection(),
		Port:                   "tcp",
		ExplicitDestinationsID: resourcetest.Resource(types.UpstreamsType, "test-servicedestinations-2").ID(),
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(proxyID): {},
		},
	}
	destCache.WriteDestination(dest1)
	destCache.WriteDestination(dest2)

	proxyCfgID := resourcetest.Resource(types.ProxyConfigurationType, "proxy-config").ID()
	proxyCfgCache.TrackProxyConfiguration(proxyCfgID, []resource.ReferenceOrID{proxyID})

	f := Fetcher{DestinationsCache: destCache, ProxyCfgCache: proxyCfgCache, Client: suite.client}
	_, err := f.FetchWorkload(context.Background(), proxyID)
	require.NoError(suite.T(), err)

	// Check that cache is updated to remove proxy id.
	require.Nil(suite.T(), destCache.DestinationsBySourceProxy(proxyID))
	require.Nil(suite.T(), proxyCfgCache.ProxyConfigurationsByProxyID(proxyID))
}

func (suite *dataFetcherSuite) TestFetcher_NotFound() {
	// This test checks that we ignore not found errors for various types we need to fetch.

	f := Fetcher{
		Client: suite.client,
	}

	cases := map[string]struct {
		typ       *pbresource.Type
		fetchFunc func(id *pbresource.ID) error
	}{
		"proxy state template": {
			typ: types.ProxyStateTemplateType,
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchProxyStateTemplate(context.Background(), id)
				return err
			},
		},
		"service endpoints": {
			typ: catalog.ServiceEndpointsType,
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchServiceEndpoints(context.Background(), id)
				return err
			},
		},
		"destinations": {
			typ: types.UpstreamsType,
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchDestinations(context.Background(), id)
				return err
			},
		},
		"service": {
			typ: catalog.ServiceType,
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchService(context.Background(), id)
				return err
			},
		},
	}

	for name, c := range cases {
		suite.T().Run(name, func(t *testing.T) {
			err := c.fetchFunc(resourcetest.Resource(c.typ, "not-found").ID())
			require.NoError(t, err)
		})
	}
}

func (suite *dataFetcherSuite) TestFetcher_FetchErrors() {
	f := Fetcher{
		Client: suite.client,
	}

	cases := map[string]struct {
		name      string
		fetchFunc func(id *pbresource.ID) error
	}{
		"workload": {
			name: "web-abc",
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchWorkload(context.Background(), id)
				return err
			},
		},
		"proxy state template": {
			name: "web-abc",
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchProxyStateTemplate(context.Background(), id)
				return err
			},
		},
		"service endpoints": {
			name: "api-1",
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchServiceEndpoints(context.Background(), id)
				return err
			},
		},
		"destinations": {
			name: "web-destinations",
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchDestinations(context.Background(), id)
				return err
			},
		},
		"service": {
			name: "web-service",
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchService(context.Background(), id)
				return err
			},
		},
	}

	for name, c := range cases {
		suite.T().Run(name+"-read", func(t *testing.T) {
			badType := &pbresource.Type{
				Group:        "not",
				Kind:         "found",
				GroupVersion: "vfake",
			}
			err := c.fetchFunc(resourcetest.Resource(badType, c.name).ID())
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})

		suite.T().Run(name+"-unmarshal", func(t *testing.T) {
			// Create a dummy health checks type as it won't be any of the types mesh controller cares about
			resourcetest.Resource(catalog.HealthChecksType, c.name).
				WithData(suite.T(), &pbcatalog.HealthChecks{
					Workloads: &pbcatalog.WorkloadSelector{Names: []string{"web-abc"}},
				}).
				Write(suite.T(), suite.client)

			err := c.fetchFunc(resourcetest.Resource(catalog.HealthChecksType, c.name).ID())
			require.Error(t, err)
			var parseErr resource.ErrDataParse
			require.ErrorAs(t, err, &parseErr)
		})
	}
}

func (suite *dataFetcherSuite) TestFetcher_FetchExplicitDestinationsData() {
	destination1 := intermediate.CombinedDestinationRef{
		ServiceRef:             resource.Reference(suite.api1Service.Id, ""),
		Port:                   "tcp",
		ExplicitDestinationsID: suite.webDestinations.Id,
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(suite.webProxy.Id): {},
		},
	}
	destination2 := intermediate.CombinedDestinationRef{
		ServiceRef:             resource.Reference(suite.api2Service.Id, ""),
		Port:                   "tcp1",
		ExplicitDestinationsID: suite.webDestinations.Id,
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(suite.webProxy.Id): {},
		},
	}
	destination3 := intermediate.CombinedDestinationRef{
		ServiceRef:             resource.Reference(suite.api2Service.Id, ""),
		Port:                   "tcp2",
		ExplicitDestinationsID: suite.webDestinations.Id,
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(suite.webProxy.Id): {},
		},
	}

	c := sidecarproxycache.NewDestinationsCache()
	c.WriteDestination(destination1)
	c.WriteDestination(destination2)
	c.WriteDestination(destination3)

	f := Fetcher{
		DestinationsCache: c,
		Client:            suite.client,
	}

	suite.T().Run("destinations not found", func(t *testing.T) {
		destinationRefNoDestinations := intermediate.CombinedDestinationRef{
			ServiceRef:             resource.Reference(suite.api1Service.Id, ""),
			Port:                   "tcp",
			ExplicitDestinationsID: resourcetest.Resource(types.UpstreamsType, "not-found").ID(),
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(suite.webProxy.Id): {},
			},
		}
		c.WriteDestination(destinationRefNoDestinations)

		destinationRefs := []intermediate.CombinedDestinationRef{destinationRefNoDestinations}
		destinations, _, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		require.Nil(t, destinations)
		_, foundDest := c.ReadDestination(destinationRefNoDestinations.ServiceRef, destinationRefNoDestinations.Port)
		require.False(t, foundDest)
	})

	suite.T().Run("service endpoints not found", func(t *testing.T) {
		notFoundServiceRef := resourcetest.Resource(catalog.ServiceType, "not-found").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			ReferenceNoSection()
		destinationNoServiceEndpoints := intermediate.CombinedDestinationRef{
			ServiceRef:             notFoundServiceRef,
			Port:                   "tcp",
			ExplicitDestinationsID: suite.webDestinations.Id,
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(suite.webProxy.Id): {},
			},
		}
		c.WriteDestination(destinationNoServiceEndpoints)

		destinationRefs := []intermediate.CombinedDestinationRef{destinationNoServiceEndpoints}
		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		require.Nil(t, destinations)

		destinationRef := resource.IDToString(destinationNoServiceEndpoints.ExplicitDestinationsID)
		serviceRef := resource.ReferenceToString(destinationNoServiceEndpoints.ServiceRef)

		require.Len(t, statuses[destinationRef].Conditions, 1)
		require.Equal(t, statuses[destinationRef].Conditions[0],
			meshStatus.ConditionDestinationServiceNotFound(serviceRef))

		_, foundDest := c.ReadDestination(destinationNoServiceEndpoints.ServiceRef, destinationNoServiceEndpoints.Port)
		require.True(t, foundDest)
	})

	suite.T().Run("service endpoints not on mesh", func(t *testing.T) {
		apiNonMeshServiceEndpointsData := &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.1"}},
					Ports: map[string]*pbcatalog.WorkloadPort{
						"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					},
					Identity: "api-1-identity",
				},
			},
		}
		apiNonMeshServiceEndpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
			WithData(suite.T(), apiNonMeshServiceEndpointsData).Write(suite.T(), suite.client)
		destinationNonMeshServiceEndpoints := intermediate.CombinedDestinationRef{
			ServiceRef:             resource.Reference(apiNonMeshServiceEndpoints.Owner, ""),
			Port:                   "tcp",
			ExplicitDestinationsID: suite.webDestinations.Id,
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(suite.webProxy.Id): {},
			},
		}
		c.WriteDestination(destinationNonMeshServiceEndpoints)

		destinationRefs := []intermediate.CombinedDestinationRef{destinationNonMeshServiceEndpoints}
		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		require.Nil(t, destinations)

		destinationRef := resource.IDToString(destinationNonMeshServiceEndpoints.ExplicitDestinationsID)
		serviceRef := resource.ReferenceToString(destinationNonMeshServiceEndpoints.ServiceRef)

		require.Len(t, statuses[destinationRef].Conditions, 2)
		prototest.AssertElementsMatch(t, statuses[destinationRef].Conditions,
			[]*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolNotFound(serviceRef),
			})

		_, foundDest := c.ReadDestination(destinationNonMeshServiceEndpoints.ServiceRef, destinationNonMeshServiceEndpoints.Port)
		require.True(t, foundDest)
	})

	suite.T().Run("invalid destinations: destination is not on the mesh", func(t *testing.T) {
		// Update api1 to no longer be on the mesh.
		suite.api1ServiceEndpoints = resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
			WithData(suite.T(), &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					{
						Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.1"}},
						Ports: map[string]*pbcatalog.WorkloadPort{
							"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
						},
						Identity: "api-1-identity",
					},
				},
			}).Write(suite.T(), suite.client)

		destinationRefs := []intermediate.CombinedDestinationRef{destination1}

		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		serviceRef := resource.ReferenceToString(destination1.ServiceRef)
		destinationRef := resource.IDToString(destination1.ExplicitDestinationsID)
		expectedStatus := &intermediate.Status{
			ID:         suite.webDestinations.Id,
			Generation: suite.webDestinations.Generation,
			Conditions: []*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolNotFound(serviceRef),
			},
		}

		require.NoError(t, err)

		// Check that the status is generated correctly.
		prototest.AssertDeepEqual(t, expectedStatus, statuses[destinationRef])

		// Check that we didn't return any destinations.
		require.Nil(t, destinations)

		// Check that destination service is still in cache because it's still referenced from the pbmesh.Upstreams
		// resource.
		_, foundDest := c.ReadDestination(destination1.ServiceRef, destination1.Port)
		require.True(t, foundDest)

		// Update the endpoints to be mesh enabled again and check that the status is now valid.
		suite.api1ServiceEndpoints = resourcetest.Resource(catalog.ServiceEndpointsType, "api-1").
			WithData(suite.T(), suite.api1ServiceEndpointsData).Write(suite.T(), suite.client)
		expectedStatus = &intermediate.Status{
			ID:         suite.webDestinations.Id,
			Generation: suite.webDestinations.Generation,
			Conditions: []*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolFound(serviceRef),
				meshStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, destination1.Port),
			},
		}

		_, statuses, err = f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, expectedStatus, statuses[destinationRef])
	})

	suite.T().Run("invalid destinations: destination is pointing to a mesh port", func(t *testing.T) {
		// Create a destination pointing to the mesh port.
		destinationMeshDestinationPort := intermediate.CombinedDestinationRef{
			ServiceRef:             resource.Reference(suite.api1Service.Id, ""),
			Port:                   "mesh",
			ExplicitDestinationsID: suite.webDestinations.Id,
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(suite.webProxy.Id): {},
			},
		}
		c.WriteDestination(destinationMeshDestinationPort)
		destinationRefs := []intermediate.CombinedDestinationRef{destinationMeshDestinationPort}

		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		serviceRef := resource.ReferenceToString(destination1.ServiceRef)
		destinationRef := resource.IDToString(destination1.ExplicitDestinationsID)
		expectedStatus := &intermediate.Status{
			ID:         suite.webDestinations.Id,
			Generation: suite.webDestinations.Generation,
			Conditions: []*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolFound(serviceRef),
				meshStatus.ConditionMeshProtocolDestinationPort(serviceRef, destinationMeshDestinationPort.Port),
			},
		}

		require.NoError(t, err)

		// Check that the status is generated correctly.
		prototest.AssertDeepEqual(t, expectedStatus, statuses[destinationRef])

		// Check that we didn't return any destinations.
		require.Nil(t, destinations)

		// Check that destination service is still in cache because it's still referenced from the pbmesh.Upstreams
		// resource.
		_, foundDest := c.ReadDestination(destinationMeshDestinationPort.ServiceRef, destinationMeshDestinationPort.Port)
		require.True(t, foundDest)

		// Update the destination to point to a non-mesh port and check that the status is now updated.
		destinationRefs[0].Port = "tcp"
		c.WriteDestination(destinationMeshDestinationPort)
		expectedStatus = &intermediate.Status{
			ID:         suite.webDestinations.Id,
			Generation: suite.webDestinations.Generation,
			Conditions: []*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolFound(serviceRef),
				meshStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, destinationRefs[0].Port),
			},
		}

		_, statuses, err = f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, expectedStatus, statuses[destinationRef])
	})

	suite.T().Run("happy path", func(t *testing.T) {
		destinationRefs := []intermediate.CombinedDestinationRef{destination1, destination2, destination3}
		expectedDestinations := []*intermediate.Destination{
			{
				Explicit: suite.webDestinationsData.Upstreams[0],
				ServiceEndpoints: &intermediate.ServiceEndpoints{
					Resource:  suite.api1ServiceEndpoints,
					Endpoints: suite.api1ServiceEndpointsData,
				},
				Identities: []*pbresource.Reference{
					{
						Name:    "api-1-identity",
						Tenancy: suite.api1Service.Id.Tenancy,
					},
				},
			},
			{
				Explicit: suite.webDestinationsData.Upstreams[1],
				ServiceEndpoints: &intermediate.ServiceEndpoints{
					Resource:  suite.api2ServiceEndpoints,
					Endpoints: suite.api2ServiceEndpointsData,
				},
				Identities: []*pbresource.Reference{
					{
						Name:    "api-2-identity",
						Tenancy: suite.api2Service.Id.Tenancy,
					},
				},
			},
			{
				Explicit: suite.webDestinationsData.Upstreams[2],
				ServiceEndpoints: &intermediate.ServiceEndpoints{
					Resource:  suite.api2ServiceEndpoints,
					Endpoints: suite.api2ServiceEndpointsData,
				},
				Identities: []*pbresource.Reference{
					{
						Name:    "api-2-identity",
						Tenancy: suite.api2Service.Id.Tenancy,
					},
				},
			},
		}
		var expectedConditions []*pbresource.Condition
		for _, d := range destinationRefs {
			ref := resource.ReferenceToString(d.ServiceRef)
			expectedConditions = append(expectedConditions,
				meshStatus.ConditionDestinationServiceFound(ref),
				meshStatus.ConditionMeshProtocolFound(ref),
				meshStatus.ConditionNonMeshProtocolDestinationPort(ref, d.Port))
		}

		actualDestinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)

		// Check that all statuses have "happy" conditions.
		dref := resource.IDToString(destination1.ExplicitDestinationsID)
		prototest.AssertElementsMatch(t, expectedConditions, statuses[dref].Conditions)

		// Check that we've computed expanded destinations correctly.
		prototest.AssertElementsMatch(t, expectedDestinations, actualDestinations)
	})
}

func (suite *dataFetcherSuite) TestFetcher_FetchImplicitDestinationsData() {
	existingDestinations := []*intermediate.Destination{
		{
			Explicit: suite.webDestinationsData.Upstreams[0],
			ServiceEndpoints: &intermediate.ServiceEndpoints{
				Resource:  suite.api1ServiceEndpoints,
				Endpoints: suite.api1ServiceEndpointsData,
			},
			Identities: []*pbresource.Reference{
				{
					Name:    "api-1-identity",
					Tenancy: suite.api1Service.Id.Tenancy,
				},
			},
		},
		{
			Explicit: suite.webDestinationsData.Upstreams[1],
			ServiceEndpoints: &intermediate.ServiceEndpoints{
				Resource:  suite.api2ServiceEndpoints,
				Endpoints: suite.api2ServiceEndpointsData,
			},
			Identities: []*pbresource.Reference{
				{
					Name:    "api-2-identity",
					Tenancy: suite.api2Service.Id.Tenancy,
				},
			},
		},
		{
			Explicit: suite.webDestinationsData.Upstreams[2],
			ServiceEndpoints: &intermediate.ServiceEndpoints{
				Resource:  suite.api2ServiceEndpoints,
				Endpoints: suite.api2ServiceEndpointsData,
			},
			Identities: []*pbresource.Reference{
				{
					Name:    "api-2-identity",
					Tenancy: suite.api2Service.Id.Tenancy,
				},
			},
		},
	}

	// Create a few other services to be implicit upstreams.
	api3Service := resourcetest.Resource(catalog.ServiceType, "api-3").
		WithData(suite.T(), &pbcatalog.Service{
			VirtualIps: []string{"192.1.1.1"},
		}).
		Write(suite.T(), suite.client)

	api3ServiceEndpointsData := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: &pbresource.ID{
					Name:    "api-3-abc",
					Tenancy: api3Service.Id.Tenancy,
					Type:    catalog.WorkloadType,
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
	api3ServiceEndpoints := resourcetest.Resource(catalog.ServiceEndpointsType, "api-3").
		WithData(suite.T(), api3ServiceEndpointsData).Write(suite.T(), suite.client)

	f := Fetcher{
		Client: suite.client,
	}

	expDestinations := append(existingDestinations, &intermediate.Destination{
		ServiceEndpoints: &intermediate.ServiceEndpoints{
			Resource:  api3ServiceEndpoints,
			Endpoints: api3ServiceEndpointsData,
		},
		Identities: []*pbresource.Reference{
			{
				Name:    "api-3-identity",
				Tenancy: api3Service.Id.Tenancy,
			},
		},
		VirtualIPs: []string{"192.1.1.1"},
	})

	actualDestinations, err := f.FetchImplicitDestinationsData(context.Background(), suite.webProxy.Id, existingDestinations)
	require.NoError(suite.T(), err)

	prototest.AssertElementsMatch(suite.T(), expDestinations, actualDestinations)
}

func (suite *dataFetcherSuite) TestFetcher_FetchAndMergeProxyConfigurations() {
	// Create some proxy configurations.
	proxyCfg1Data := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
		},
	}

	proxyCfg2Data := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			MutualTlsMode: pbmesh.MutualTLSMode_MUTUAL_TLS_MODE_DEFAULT,
		},
	}

	proxyCfg1 := resourcetest.Resource(types.ProxyConfigurationType, "config-1").
		WithData(suite.T(), proxyCfg1Data).
		Write(suite.T(), suite.client)

	proxyCfg2 := resourcetest.Resource(types.ProxyConfigurationType, "config-2").
		WithData(suite.T(), proxyCfg2Data).
		Write(suite.T(), suite.client)

	proxyCfgCache := sidecarproxycache.NewProxyConfigurationCache()
	proxyCfgCache.TrackProxyConfiguration(proxyCfg1.Id, []resource.ReferenceOrID{suite.webProxy.Id})
	proxyCfgCache.TrackProxyConfiguration(proxyCfg2.Id, []resource.ReferenceOrID{suite.webProxy.Id})

	expectedProxyCfg := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode:          pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			MutualTlsMode: pbmesh.MutualTLSMode_MUTUAL_TLS_MODE_DEFAULT,
		},
	}

	fetcher := Fetcher{Client: suite.client, ProxyCfgCache: proxyCfgCache}

	actualProxyCfg, err := fetcher.FetchAndMergeProxyConfigurations(suite.ctx, suite.webProxy.Id)
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), expectedProxyCfg, actualProxyCfg)

	// Delete proxy cfg and check that the cache gets updated.
	_, err = suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: proxyCfg1.Id})
	require.NoError(suite.T(), err)

	_, err = fetcher.FetchAndMergeProxyConfigurations(suite.ctx, suite.webProxy.Id)
	require.NoError(suite.T(), err)

	proxyCfg2.Id.Uid = ""
	prototest.AssertElementsMatch(suite.T(),
		[]*pbresource.ID{proxyCfg2.Id},
		fetcher.ProxyCfgCache.ProxyConfigurationsByProxyID(suite.webProxy.Id))
}

func TestDataFetcher(t *testing.T) {
	suite.Run(t, new(dataFetcherSuite))
}
