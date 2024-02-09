// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

type dataFetcherSuite struct {
	suite.Suite

	ctx    context.Context
	client *resourcetest.Client
	rt     controller.Runtime

	ctl       *controller.TestController
	tenancies []*pbresource.Tenancy

	api1Service                 *pbresource.Resource
	api1ServiceData             *pbcatalog.Service
	api2Service                 *pbresource.Resource
	api2ServiceData             *pbcatalog.Service
	webComputedDestinationsData *pbmesh.ComputedExplicitDestinations
	webProxy                    *pbresource.Resource
	webWorkload                 *pbresource.Resource
}

func (suite *dataFetcherSuite) SetupTest() {
	trustDomainFetcher := func() (string, error) { return "test.consul", nil }

	suite.tenancies = resourcetest.TestTenancies()

	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes, auth.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.ctl = controller.NewTestController(Controller(trustDomainFetcher, "dc1", false), client).
		WithLogger(testutil.Logger(suite.T()))
	suite.rt = suite.ctl.Runtime()
	suite.client = resourcetest.NewClient(suite.rt.Client)
}

func (suite *dataFetcherSuite) setupWithTenancy(tenancy *pbresource.Tenancy) {
	suite.api1Service, suite.api1ServiceData, _ = suite.createService(
		"api-1", tenancy, "api1", []*pbcatalog.ServicePort{
			{TargetPort: "tcp", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		}, nil, []string{"api-1-identity"}, false)

	suite.api2Service, suite.api2ServiceData, _ = suite.createService(
		"api-2", tenancy, "api2", []*pbcatalog.ServicePort{
			{TargetPort: "tcp1", VirtualPort: 9080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "tcp2", VirtualPort: 9081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		}, nil, []string{"api-2-identity"}, false)

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
		WithTenancy(tenancy).
		WithData(suite.T(), &pbmesh.ProxyStateTemplate{}).
		Write(suite.T(), suite.client)

	suite.webWorkload = resourcetest.Resource(pbcatalog.WorkloadType, "web-abc").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "10.0.0.2"}},
			Ports:     map[string]*pbcatalog.WorkloadPort{"tcp": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP}},
			Identity:  "web-id-abc",
		}).
		Write(suite.T(), suite.client)
}

func (suite *dataFetcherSuite) TestFetcher_FetchExplicitDestinationsData() {
	const mgwMode = pbmesh.MeshGatewayMode_MESH_GATEWAY_MODE_NONE

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {

		api1ServiceRef := resource.Reference(suite.api1Service.Id, "")

		webWrk := resourcetest.MustDecode[*pbcatalog.Workload](suite.T(), suite.webWorkload)

		testutil.RunStep(suite.T(), "computed destinations not found", func(t *testing.T) {
			// We will try to fetch explicit destinations for a proxy that doesn't have one.
			destinations, err := fetchComputedExplicitDestinationsData(suite.rt, webWrk, mgwMode)
			require.NoError(t, err)
			require.Nil(t, destinations)
		})

		testutil.RunStep(suite.T(), "invalid destinations: service not found", func(t *testing.T) {
			notFoundServiceRef := resourcetest.Resource(pbcatalog.ServiceType, "not-found").
				WithTenancy(tenancy).
				ReferenceNoSection()

			resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
				WithData(t, &pbmesh.ComputedExplicitDestinations{
					Destinations: []*pbmesh.Destination{
						{
							DestinationRef:  notFoundServiceRef,
							DestinationPort: "tcp",
						},
					},
				}).
				WithTenancy(tenancy).
				Write(t, suite.client)

			destinations, err := fetchComputedExplicitDestinationsData(suite.rt, webWrk, mgwMode)
			require.NoError(t, err)
			require.Nil(t, destinations)
		})

		testutil.RunStep(suite.T(), "invalid destinations: service not on mesh", func(t *testing.T) {
			apiNonMeshServiceData := &pbcatalog.Service{
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			}
			resourcetest.ResourceID(suite.api1Service.Id).
				WithTenancy(tenancy).
				WithData(t, apiNonMeshServiceData).
				Write(t, suite.client)
			resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
				WithData(t, &pbmesh.ComputedExplicitDestinations{
					Destinations: []*pbmesh.Destination{
						{
							DestinationRef:  api1ServiceRef,
							DestinationPort: "tcp",
						},
					},
				}).
				WithTenancy(tenancy).
				Write(t, suite.client)

			destinations, err := fetchComputedExplicitDestinationsData(suite.rt, webWrk, mgwMode)
			require.NoError(t, err)
			require.Nil(t, destinations)
		})

		testutil.RunStep(suite.T(), "invalid destinations: destination port not found", func(t *testing.T) {
			suite.api1Service, _, _ = suite.createService(
				"api-1", tenancy, "api1", []*pbcatalog.ServicePort{
					{TargetPort: "some-other-port", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				}, nil, []string{"api-1-identity"}, false)
			resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
				WithData(t, &pbmesh.ComputedExplicitDestinations{
					Destinations: []*pbmesh.Destination{
						{
							DestinationRef:  api1ServiceRef,
							DestinationPort: "tcp",
						},
					},
				}).
				WithTenancy(tenancy).
				Write(t, suite.client)

			destinations, err := fetchComputedExplicitDestinationsData(suite.rt, webWrk, mgwMode)
			require.NoError(t, err)
			require.Nil(t, destinations)
		})

		suite.api1Service = resourcetest.ResourceID(suite.api1Service.Id).
			WithTenancy(tenancy).
			WithData(suite.T(), suite.api1ServiceData).
			Write(suite.T(), suite.client)

		suite.api2Service = resourcetest.ResourceID(suite.api2Service.Id).
			WithTenancy(tenancy).
			WithData(suite.T(), suite.api2ServiceData).
			Write(suite.T(), suite.client)

		testutil.RunStep(suite.T(), "invalid destinations: destination is pointing to a mesh port", func(t *testing.T) {
			// Create a computed destinations resource pointing to the mesh port.
			resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
				WithData(t, &pbmesh.ComputedExplicitDestinations{
					Destinations: []*pbmesh.Destination{
						{
							DestinationRef:  api1ServiceRef,
							DestinationPort: "mesh",
						},
					},
				}).
				WithTenancy(tenancy).
				Write(t, suite.client)

			destinations, err := fetchComputedExplicitDestinationsData(suite.rt, webWrk, mgwMode)
			require.NoError(t, err)
			require.Empty(t, destinations)
		})

		resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.webProxy.Id.Name).
			WithData(suite.T(), suite.webComputedDestinationsData).
			WithTenancy(tenancy).
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
				WithTenancy(tenancy).
				Build()

			api1ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, suite.api1Service.Id)
			api1ComputedRoutes := routestest.ReconcileComputedRoutes(suite.T(), suite.client, api1ComputedRoutesID,
				resourcetest.MustDecode[*pbcatalog.Service](suite.T(), apiNonTCPService),
			)
			require.NotNil(suite.T(), api1ComputedRoutes)

			// This destination points to TCP, but the computed routes is stale and only knows about HTTP.
			destinations, err := fetchComputedExplicitDestinationsData(suite.rt, webWrk, mgwMode)
			require.NoError(t, err)

			// Check that we didn't return any destinations.
			require.Nil(t, destinations)
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

			expectedDestinations := []*intermediate.Destination{
				{
					Explicit: suite.webComputedDestinationsData.Destinations[0],
					Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api1Service),
					ComputedPortRoutes: routestest.MutateTargets(suite.T(), api1ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
						switch {
						case resource.ReferenceOrIDMatch(suite.api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
							details.ServiceEndpointsRef = &pbproxystate.EndpointRef{
								Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, suite.api1Service.Id),
								MeshPort:  details.MeshPort,
								RoutePort: details.BackendRef.Port,
							}
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
							details.ServiceEndpointsRef = &pbproxystate.EndpointRef{
								Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, suite.api2Service.Id),
								MeshPort:  details.MeshPort,
								RoutePort: details.BackendRef.Port,
							}
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
							details.ServiceEndpointsRef = &pbproxystate.EndpointRef{
								Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, suite.api2Service.Id),
								MeshPort:  details.MeshPort,
								RoutePort: details.BackendRef.Port,
							}
							details.IdentityRefs = []*pbresource.Reference{{
								Name:    "api-2-identity",
								Tenancy: suite.api2Service.Id.Tenancy,
							}}
						}
					}),
				},
			}

			actualDestinations, err := fetchComputedExplicitDestinationsData(suite.rt, webWrk, mgwMode)
			require.NoError(t, err)

			// Check that we've computed expanded destinations correctly.
			prototest.AssertElementsMatch(t, expectedDestinations, actualDestinations)
		})
	})
}

func (suite *dataFetcherSuite) TestFetcher_FetchImplicitDestinationsData() {
	const mgwMode = pbmesh.MeshGatewayMode_MESH_GATEWAY_MODE_NONE

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		webWrk := resourcetest.MustDecode[*pbcatalog.Workload](suite.T(), suite.webWorkload)

		// Create a few other services to be implicit upstreams.
		api3Service, _, _ := suite.createService(
			"api-3", tenancy, "api3", []*pbcatalog.ServicePort{
				{TargetPort: "tcp", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			}, []string{"192.1.1.1"}, []string{"api-3-identity"}, false)

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

		cidID := &pbresource.ID{
			Type:    pbmesh.ComputedImplicitDestinationsType,
			Tenancy: webWrk.Id.Tenancy,
			Name:    webWrk.Data.Identity,
		}

		// Write a CID that grants web-id-abc access to api-3-identity
		resourcetest.ResourceID(cidID).
			WithTenancy(tenancy).
			WithData(suite.T(), &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{
						DestinationRef:   resource.Reference(api3Service.Id, ""),
						DestinationPorts: []string{"tcp"},
					},
				},
			}).
			Write(suite.T(), suite.client)

		existingDestinations := []*intermediate.Destination{
			{
				Explicit: suite.webComputedDestinationsData.Destinations[0],
				Service:  resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.api1Service),
				ComputedPortRoutes: routestest.MutateTargets(suite.T(), api1ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
					switch {
					case resource.ReferenceOrIDMatch(suite.api1Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
						details.ServiceEndpointsRef = &pbproxystate.EndpointRef{
							Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, suite.api1Service.Id),
							MeshPort:  details.MeshPort,
							RoutePort: details.BackendRef.Port,
						}
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
						details.ServiceEndpointsRef = &pbproxystate.EndpointRef{
							Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, suite.api2Service.Id),
							MeshPort:  details.MeshPort,
							RoutePort: details.BackendRef.Port,
						}
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
						details.ServiceEndpointsRef = &pbproxystate.EndpointRef{
							Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, suite.api2Service.Id),
							MeshPort:  details.MeshPort,
							RoutePort: details.BackendRef.Port,
						}
						details.IdentityRefs = []*pbresource.Reference{{
							Name:    "api-2-identity",
							Tenancy: suite.api1Service.Id.Tenancy,
						}}
					}
				}),
			},
		}

		actualDestinations, err := fetchComputedImplicitDestinationsData(
			suite.rt, webWrk, mgwMode, slices.Clone(existingDestinations),
		)
		require.NoError(suite.T(), err)

		existingDestinations = append(existingDestinations, &intermediate.Destination{
			// implicit
			Service: resourcetest.MustDecode[*pbcatalog.Service](suite.T(), api3Service),
			ComputedPortRoutes: routestest.MutateTargets(suite.T(), api3ComputedRoutes.Data, "tcp", func(t *testing.T, details *pbmesh.BackendTargetDetails) {
				switch {
				case resource.ReferenceOrIDMatch(api3Service.Id, details.BackendRef.Ref) && details.BackendRef.Port == "tcp":
					details.ServiceEndpointsRef = &pbproxystate.EndpointRef{
						Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, api3Service.Id),
						MeshPort:  details.MeshPort,
						RoutePort: details.BackendRef.Port,
					}
					details.IdentityRefs = []*pbresource.Reference{{
						Name:    "api-3-identity",
						Tenancy: suite.api1Service.Id.Tenancy,
					}}
				}
			}),
			VirtualIPs: []string{"192.1.1.1"},
		})
		prototest.AssertElementsMatch(suite.T(), existingDestinations, actualDestinations)
	})
}

func TestDataFetcher(t *testing.T) {
	suite.Run(t, new(dataFetcherSuite))
}

func (suite *dataFetcherSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *dataFetcherSuite) cleanUpNodes() {
	suite.client.MustDelete(suite.T(), suite.api1Service.Id)
	suite.client.MustDelete(suite.T(), suite.api2Service.Id)
	suite.client.MustDelete(suite.T(), suite.webProxy.Id)
	suite.client.MustDelete(suite.T(), suite.webWorkload.Id)
}

func (suite *dataFetcherSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupWithTenancy(tenancy)
			suite.T().Cleanup(func() {
				suite.cleanUpNodes()
			})
			t(tenancy)
		})
	}
}

func (suite *dataFetcherSuite) createService(
	name string,
	tenancy *pbresource.Tenancy,
	exactSelector string,
	ports []*pbcatalog.ServicePort,
	vips []string,
	workloadIdentities []string,
	deferStatusUpdate bool,
) (*pbresource.Resource, *pbcatalog.Service, func() *pbresource.Resource) {
	return createService(suite.T(), suite.client, name, tenancy, exactSelector, ports, vips, workloadIdentities, deferStatusUpdate)
}
