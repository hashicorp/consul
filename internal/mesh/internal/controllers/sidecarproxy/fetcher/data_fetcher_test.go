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
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	meshStatus "github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
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

func TestIsMeshEnabled(t *testing.T) {
	cases := map[string]struct {
		ports []*pbcatalog.ServicePort
		exp   bool
	}{
		"nil ports": {
			ports: nil,
			exp:   false,
		},
		"empty ports": {
			ports: []*pbcatalog.ServicePort{},
			exp:   false,
		},
		"no mesh ports": {
			ports: []*pbcatalog.ServicePort{
				{VirtualPort: 1000, TargetPort: "p1", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{VirtualPort: 2000, TargetPort: "p2", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			},
			exp: false,
		},
		"one mesh port": {
			ports: []*pbcatalog.ServicePort{
				{VirtualPort: 1000, TargetPort: "p1", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{VirtualPort: 2000, TargetPort: "p2", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{VirtualPort: 3000, TargetPort: "p3", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
			exp: true,
		},
		"multiple mesh ports": {
			ports: []*pbcatalog.ServicePort{
				{VirtualPort: 1000, TargetPort: "p1", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{VirtualPort: 2000, TargetPort: "p2", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{VirtualPort: 3000, TargetPort: "p3", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{VirtualPort: 4000, TargetPort: "p4", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
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

func TestIsWorkloadMeshEnabled(t *testing.T) {
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
			require.Equal(t, c.exp, IsWorkloadMeshEnabled(c.ports))
		})
	}
}

type dataFetcherSuite struct {
	suite.Suite

	ctx    context.Context
	client pbresource.ResourceServiceClient
	rt     controller.Runtime

	api1Service              *pbresource.Resource
	api1ServiceData          *pbcatalog.Service
	api2Service              *pbresource.Resource
	api2ServiceData          *pbcatalog.Service
	api1ServiceEndpoints     *pbresource.Resource
	api1ServiceEndpointsData *pbcatalog.ServiceEndpoints
	api2ServiceEndpoints     *pbresource.Resource
	api2ServiceEndpointsData *pbcatalog.ServiceEndpoints
	webDestinations          *pbresource.Resource
	webDestinationsData      *pbmesh.Destinations
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

	suite.webDestinationsData = &pbmesh.Destinations{
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

	suite.webDestinations = resourcetest.Resource(pbmesh.DestinationsType, "web-destinations").
		WithData(suite.T(), suite.webDestinationsData).
		Write(suite.T(), suite.client)

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
	proxyID := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "service-workload-abc").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		ID()
	identityID := resourcetest.Resource(pbauth.WorkloadIdentityType, "workload-identity-abc").ID()

	// Create cache and pre-populate it.
	var (
		destCache           = sidecarproxycache.NewDestinationsCache()
		proxyCfgCache       = sidecarproxycache.NewProxyConfigurationCache()
		computedRoutesCache = sidecarproxycache.NewComputedRoutesCache()
		identitiesCache     = sidecarproxycache.NewIdentitiesCache()
	)

	f := Fetcher{
		DestinationsCache:   destCache,
		ProxyCfgCache:       proxyCfgCache,
		ComputedRoutesCache: computedRoutesCache,
		IdentitiesCache:     identitiesCache,
		Client:              suite.client,
	}

	// Prepopulate the cache.
	dest1 := intermediate.CombinedDestinationRef{
		ServiceRef:             resourcetest.Resource(pbcatalog.ServiceType, "test-service-1").ReferenceNoSection(),
		Port:                   "tcp",
		ExplicitDestinationsID: resourcetest.Resource(pbmesh.DestinationsType, "test-servicedestinations-1").ID(),
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(proxyID): {},
		},
	}
	dest2 := intermediate.CombinedDestinationRef{
		ServiceRef:             resourcetest.Resource(pbcatalog.ServiceType, "test-service-2").ReferenceNoSection(),
		Port:                   "tcp",
		ExplicitDestinationsID: resourcetest.Resource(pbmesh.DestinationsType, "test-servicedestinations-2").ID(),
		SourceProxies: map[resource.ReferenceKey]struct{}{
			resource.NewReferenceKey(proxyID): {},
		},
	}

	destCache.WriteDestination(dest1)
	destCache.WriteDestination(dest2)
	suite.syncDestinations(dest1, dest2)

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

	// Track the workload's identity
	_, err := f.FetchWorkload(context.Background(), workload.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), destCache.DestinationsBySourceProxy(proxyID))
	require.Nil(suite.T(), proxyCfgCache.ProxyConfigurationsByProxyID(proxyID))
	require.Nil(suite.T(), proxyCfgCache.ProxyConfigurationsByProxyID(proxyID))
	require.Equal(suite.T(), []*pbresource.ID{proxyID}, identitiesCache.ProxyIDsByWorkloadIdentity(identityID))

	proxyCfgID := resourcetest.Resource(pbmesh.ProxyConfigurationType, "proxy-config").ID()
	proxyCfgCache.TrackProxyConfiguration(proxyCfgID, []resource.ReferenceOrID{proxyID})

	_, err = f.FetchWorkload(context.Background(), proxyID)
	require.NoError(suite.T(), err)

	// Check that cache is updated to remove proxy id.
	require.Nil(suite.T(), destCache.DestinationsBySourceProxy(proxyID))
	require.Nil(suite.T(), proxyCfgCache.ProxyConfigurationsByProxyID(proxyID))
	require.Nil(suite.T(), proxyCfgCache.ProxyConfigurationsByProxyID(proxyID))
	require.Nil(suite.T(), identitiesCache.ProxyIDsByWorkloadIdentity(identityID))
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
			typ: pbmesh.ProxyStateTemplateType,
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchProxyStateTemplate(context.Background(), id)
				return err
			},
		},
		"service endpoints": {
			typ: pbcatalog.ServiceEndpointsType,
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchServiceEndpoints(context.Background(), id)
				return err
			},
		},
		"destinations": {
			typ: pbmesh.DestinationsType,
			fetchFunc: func(id *pbresource.ID) error {
				_, err := f.FetchDestinations(context.Background(), id)
				return err
			},
		},
		"service": {
			typ: pbcatalog.ServiceType,
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
			resourcetest.Resource(pbcatalog.HealthChecksType, c.name).
				WithData(suite.T(), &pbcatalog.HealthChecks{
					Workloads: &pbcatalog.WorkloadSelector{Names: []string{"web-abc"}},
				}).
				Write(suite.T(), suite.client)

			err := c.fetchFunc(resourcetest.Resource(pbcatalog.HealthChecksType, c.name).ID())
			require.Error(t, err)
			var parseErr resource.ErrDataParse
			require.ErrorAs(t, err, &parseErr)
		})
	}
}

func (suite *dataFetcherSuite) syncDestinations(destinations ...intermediate.CombinedDestinationRef) {
	data := &pbmesh.Destinations{}
	for _, dest := range destinations {
		data.Destinations = append(data.Destinations, &pbmesh.Destination{
			DestinationRef:  dest.ServiceRef,
			DestinationPort: dest.Port,
		})
	}

	suite.webDestinations = resourcetest.Resource(pbmesh.DestinationsType, "web-destinations").
		WithData(suite.T(), data).
		Write(suite.T(), suite.client)
}

func (suite *dataFetcherSuite) TestFetcher_FetchExplicitDestinationsData() {
	var (
		c       = sidecarproxycache.NewDestinationsCache()
		crCache = sidecarproxycache.NewComputedRoutesCache()
	)

	writeDestination := func(t *testing.T, dest intermediate.CombinedDestinationRef) {
		c.WriteDestination(dest)
		t.Cleanup(func() {
			c.DeleteDestination(dest.ServiceRef, dest.Port)
		})
	}

	var (
		api1ServiceRef = resource.Reference(suite.api1Service.Id, "")
	)

	f := Fetcher{
		DestinationsCache:   c,
		ComputedRoutesCache: crCache,
		Client:              suite.client,
	}

	testutil.RunStep(suite.T(), "invalid destinations: destinations not found", func(t *testing.T) {
		destinationRefNoDestinations := intermediate.CombinedDestinationRef{
			ServiceRef:             api1ServiceRef,
			Port:                   "tcp",
			ExplicitDestinationsID: resourcetest.Resource(pbmesh.DestinationsType, "not-found").ID(),
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(suite.webProxy.Id): {},
			},
		}
		c.WriteDestination(destinationRefNoDestinations)
		suite.syncDestinations(destinationRefNoDestinations)

		destinationRefs := []intermediate.CombinedDestinationRef{destinationRefNoDestinations}
		destinations, _, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		require.Nil(t, destinations)
		_, foundDest := c.ReadDestination(destinationRefNoDestinations.ServiceRef, destinationRefNoDestinations.Port)
		require.False(t, foundDest)
	})

	testutil.RunStep(suite.T(), "invalid destinations: service not found", func(t *testing.T) {
		notFoundServiceRef := resourcetest.Resource(pbcatalog.ServiceType, "not-found").
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
		suite.syncDestinations(destinationNoServiceEndpoints)

		t.Cleanup(func() {
			// Restore this for the next test step.
			suite.webDestinations = resourcetest.Resource(pbmesh.DestinationsType, "web-destinations").
				WithData(suite.T(), suite.webDestinationsData).
				Write(suite.T(), suite.client)
		})
		suite.webDestinations = resourcetest.Resource(pbmesh.DestinationsType, "web-destinations").
			WithData(suite.T(), &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{{
					DestinationRef:  notFoundServiceRef,
					DestinationPort: "tcp",
				}},
			}).
			Write(suite.T(), suite.client)

		destinationRefs := []intermediate.CombinedDestinationRef{destinationNoServiceEndpoints}
		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		require.Empty(t, destinations)

		destinationRef := resource.IDToString(destinationNoServiceEndpoints.ExplicitDestinationsID)
		serviceRef := resource.ReferenceToString(destinationNoServiceEndpoints.ServiceRef)

		destStatus, exists := statuses[destinationRef]
		require.True(t, exists, "status map does not contain service: %s", destinationRef)

		require.Len(t, destStatus.Conditions, 1)
		require.Equal(t, destStatus.Conditions[0],
			meshStatus.ConditionDestinationServiceNotFound(serviceRef))

		_, foundDest := c.ReadDestination(destinationNoServiceEndpoints.ServiceRef, destinationNoServiceEndpoints.Port)
		require.True(t, foundDest)
	})

	testutil.RunStep(suite.T(), "invalid destinations: service not on mesh", func(t *testing.T) {
		apiNonMeshServiceData := &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			},
		}
		suite.api1Service = resourcetest.ResourceID(suite.api1Service.Id).
			WithData(suite.T(), apiNonMeshServiceData).
			Write(suite.T(), suite.client)
		destinationNonMeshServiceEndpoints := intermediate.CombinedDestinationRef{
			ServiceRef:             api1ServiceRef,
			Port:                   "tcp",
			ExplicitDestinationsID: suite.webDestinations.Id,
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(suite.webProxy.Id): {},
			},
		}
		c.WriteDestination(destinationNonMeshServiceEndpoints)
		suite.syncDestinations(destinationNonMeshServiceEndpoints)

		destinationRefs := []intermediate.CombinedDestinationRef{destinationNonMeshServiceEndpoints}
		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		require.Nil(t, destinations)

		destinationRef := resource.IDToString(destinationNonMeshServiceEndpoints.ExplicitDestinationsID)
		serviceRef := resource.ReferenceToString(destinationNonMeshServiceEndpoints.ServiceRef)

		destStatus, exists := statuses[destinationRef]
		require.True(t, exists, "status map does not contain service: %s", destinationRef)

		prototest.AssertElementsMatch(t, []*pbresource.Condition{
			meshStatus.ConditionDestinationServiceFound(serviceRef),
			meshStatus.ConditionMeshProtocolNotFound(serviceRef),
		}, destStatus.Conditions)

		_, foundDest := c.ReadDestination(destinationNonMeshServiceEndpoints.ServiceRef, destinationNonMeshServiceEndpoints.Port)
		require.True(t, foundDest)

		// Update the service to be mesh enabled again and check that the status is now valid.
		suite.api1Service = resourcetest.ResourceID(suite.api1Service.Id).
			WithData(suite.T(), suite.api1ServiceData).
			Write(suite.T(), suite.client)

		destinations, statuses, err = f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)
		require.Nil(t, destinations)

		destStatus, exists = statuses[destinationRef]
		require.True(t, exists, "status map does not contain service: %s", destinationRef)

		prototest.AssertElementsMatch(t, []*pbresource.Condition{
			meshStatus.ConditionDestinationServiceFound(serviceRef),
			meshStatus.ConditionMeshProtocolFound(serviceRef),
			meshStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, destinationNonMeshServiceEndpoints.Port),
			meshStatus.ConditionDestinationComputedRoutesNotFound(serviceRef),
		}, destStatus.Conditions)
	})

	testutil.RunStep(suite.T(), "invalid destinations: destination is pointing to a mesh port", func(t *testing.T) {
		// Create a destination pointing to the mesh port.
		destinationMeshDestinationPort := intermediate.CombinedDestinationRef{
			ServiceRef:             api1ServiceRef,
			Port:                   "mesh",
			ExplicitDestinationsID: suite.webDestinations.Id,
			SourceProxies: map[resource.ReferenceKey]struct{}{
				resource.NewReferenceKey(suite.webProxy.Id): {},
			},
		}
		c.WriteDestination(destinationMeshDestinationPort)
		suite.syncDestinations(destinationMeshDestinationPort)
		destinationRefs := []intermediate.CombinedDestinationRef{destinationMeshDestinationPort}

		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		serviceRef := resource.ReferenceToString(destinationMeshDestinationPort.ServiceRef)
		destinationRef := resource.IDToString(destinationMeshDestinationPort.ExplicitDestinationsID)
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

		destStatus, exists := statuses[destinationRef]
		require.True(t, exists, "status map does not contain service: %s", destinationRef)

		// Check that the status is generated correctly.
		prototest.AssertDeepEqual(t, expectedStatus, destStatus)

		// Check that we didn't return any destinations.
		require.Empty(t, destinations)

		// Check that destination service is still in cache because it's still referenced from the pbmesh.Destinations
		// resource.
		_, foundDest := c.ReadDestination(destinationMeshDestinationPort.ServiceRef, destinationMeshDestinationPort.Port)
		require.True(t, foundDest)

		// Update the destination to point to a non-mesh port and check that the status is now updated.
		destinationRefs[0].Port = "tcp"
		c.WriteDestination(destinationMeshDestinationPort)
		suite.syncDestinations(destinationMeshDestinationPort)
		expectedStatus = &intermediate.Status{
			ID:         suite.webDestinations.Id,
			Generation: suite.webDestinations.Generation,
			Conditions: []*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolFound(serviceRef),
				meshStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, destinationRefs[0].Port),
				meshStatus.ConditionDestinationComputedRoutesNotFound(serviceRef),
			},
		}

		_, statuses, err = f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)

		destStatus, exists = statuses[destinationRef]
		require.True(t, exists, "status map does not contain service: %s", destinationRef)

		prototest.AssertDeepEqual(t, expectedStatus, destStatus)
	})

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

	testutil.RunStep(suite.T(), "invalid destinations: destination is pointing to a port but computed routes is not aware of it yet", func(t *testing.T) {
		apiNonTCPServiceData := &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}
		apiNonTCPService := resourcetest.ResourceID(suite.api1Service.Id).
			WithData(suite.T(), apiNonTCPServiceData).
			Build()

		api1ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, suite.api1Service.Id)
		api1ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api1ComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), apiNonTCPService),
		)
		require.NotNil(suite.T(), api1ComputedRoutes)

		// This destination points to TCP, but the computed routes is stale and only knows about HTTP.
		writeDestination(t, destination1)
		suite.syncDestinations(destination1)
		destinationRefs := []intermediate.CombinedDestinationRef{destination1}

		destinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		serviceRef := resource.ReferenceToString(destination1.ServiceRef)
		destinationRef := resource.IDToString(destination1.ExplicitDestinationsID)
		expectedStatus := &intermediate.Status{
			ID:         suite.webDestinations.Id,
			Generation: suite.webDestinations.Generation,
			Conditions: []*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolFound(serviceRef),
				meshStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, destination1.Port),
				meshStatus.ConditionDestinationComputedRoutesFound(serviceRef),
				meshStatus.ConditionDestinationComputedRoutesPortNotFound(serviceRef, destination1.Port),
			},
		}

		require.NoError(t, err)

		destStatus, exists := statuses[destinationRef]
		require.True(t, exists, "status map does not contain service: %s", destinationRef)

		// Check that the status is generated correctly.
		prototest.AssertDeepEqual(t, expectedStatus, destStatus)

		// Check that we didn't return any destinations.
		require.Nil(t, destinations)

		// Check that destination service is still in cache because it's still referenced from the pbmesh.Destinations
		// resource.
		_, foundDest := c.ReadDestination(destination1.ServiceRef, destination1.Port)
		require.True(t, foundDest)

		// Update the computed routes not not lag.
		api1ComputedRoutes = routestest.ReconcileComputedRoutes(suite.T(), suite.client, api1ComputedRoutesID,
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api1Service),
		)
		require.NotNil(suite.T(), api1ComputedRoutes)

		expectedStatus = &intermediate.Status{
			ID:         suite.webDestinations.Id,
			Generation: suite.webDestinations.Generation,
			Conditions: []*pbresource.Condition{
				meshStatus.ConditionDestinationServiceFound(serviceRef),
				meshStatus.ConditionMeshProtocolFound(serviceRef),
				meshStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, destination1.Port),
				meshStatus.ConditionDestinationComputedRoutesFound(serviceRef),
				meshStatus.ConditionDestinationComputedRoutesPortFound(serviceRef, destination1.Port),
			},
		}

		actualDestinations, statuses, err := f.FetchExplicitDestinationsData(suite.ctx, destinationRefs)
		require.NoError(t, err)

		destStatus, exists = statuses[destinationRef]
		require.True(t, exists, "status map does not contain service: %s", destinationRef)

		prototest.AssertDeepEqual(t, expectedStatus, destStatus)

		expectedDestinations := []*intermediate.Destination{
			{
				Explicit: suite.webDestinationsData.Destinations[0],
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
		}
		prototest.AssertElementsMatch(t, expectedDestinations, actualDestinations)
	})

	testutil.RunStep(suite.T(), "happy path", func(t *testing.T) {
		writeDestination(t, destination1)
		writeDestination(t, destination2)
		writeDestination(t, destination3)
		suite.syncDestinations(destination1, destination2, destination3)

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

		destinationRefs := []intermediate.CombinedDestinationRef{destination1, destination2, destination3}
		expectedDestinations := []*intermediate.Destination{
			{
				Explicit: suite.webDestinationsData.Destinations[0],
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
				Explicit: suite.webDestinationsData.Destinations[1],
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
				Explicit: suite.webDestinationsData.Destinations[2],
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
		var expectedConditions []*pbresource.Condition
		for _, d := range destinationRefs {
			ref := resource.ReferenceToString(d.ServiceRef)
			expectedConditions = append(expectedConditions,
				meshStatus.ConditionDestinationServiceFound(ref),
				meshStatus.ConditionMeshProtocolFound(ref),
				meshStatus.ConditionNonMeshProtocolDestinationPort(ref, d.Port),
				meshStatus.ConditionDestinationComputedRoutesFound(ref),
				meshStatus.ConditionDestinationComputedRoutesPortFound(ref, d.Port),
			)
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
			Explicit: suite.webDestinationsData.Destinations[0],
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
			Explicit: suite.webDestinationsData.Destinations[1],
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
			Explicit: suite.webDestinationsData.Destinations[2],
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
		Client: suite.client,
	}

	actualDestinations, err := f.FetchImplicitDestinationsData(context.Background(), suite.webProxy.Id, existingDestinations)
	require.NoError(suite.T(), err)

	prototest.AssertElementsMatch(suite.T(), existingDestinations, actualDestinations)
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

	proxyCfg1 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "config-1").
		WithData(suite.T(), proxyCfg1Data).
		Write(suite.T(), suite.client)

	proxyCfg2 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "config-2").
		WithData(suite.T(), proxyCfg2Data).
		Write(suite.T(), suite.client)

	proxyCfgCache := sidecarproxycache.NewProxyConfigurationCache()
	proxyCfgCache.TrackProxyConfiguration(proxyCfg1.Id, []resource.ReferenceOrID{suite.webProxy.Id})
	proxyCfgCache.TrackProxyConfiguration(proxyCfg2.Id, []resource.ReferenceOrID{suite.webProxy.Id})

	expectedProxyCfg := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode:          pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			MutualTlsMode: pbmesh.MutualTLSMode_MUTUAL_TLS_MODE_DEFAULT,
			TransparentProxy: &pbmesh.TransparentProxy{
				OutboundListenerPort: 15001,
			},
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
