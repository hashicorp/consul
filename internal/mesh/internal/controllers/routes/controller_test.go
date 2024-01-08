// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type controllerSuite struct {
	suite.Suite

	ctx       context.Context
	client    *rtest.Client
	rt        controller.Runtime
	tenancies []*pbresource.Tenancy

	refs *testResourceRef
}

type testResourceRef struct {
	apiServiceRef *pbresource.Reference
	fooServiceRef *pbresource.Reference
	barServiceRef *pbresource.Reference
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = rtest.TestTenancies()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.rt = controller.Runtime{
		Client: client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.client = rtest.NewClient(client)
}

func (suite *controllerSuite) TestController() {
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	suite.runTestCaseWithTenancies(func(refs *testResourceRef) {

		backendName := func(name, port string, tenancy *pbresource.Tenancy) string {
			return fmt.Sprintf("catalog.v2beta1.Service/%s.local.%s/%s?port=%s", tenancy.Partition, tenancy.Namespace, name, port)
		}

		var (
			apiServiceRef = refs.apiServiceRef
			fooServiceRef = refs.fooServiceRef
			barServiceRef = refs.barServiceRef

			computedRoutesID = rtest.Resource(pbmesh.ComputedRoutesType, "api").
						WithTenancy(apiServiceRef.Tenancy).
						ID()
		)

		// Start out by creating a single port service and let it create the
		// default computed routes for tcp.

		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				// {TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		_ = rtest.Resource(pbcatalog.ServiceType, "api").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), apiServiceData).
			Write(suite.T(), suite.client)

		var lastVersion string
		testutil.RunStep(suite.T(), "default tcp route", func(t *testing.T) {
			// Check that the computed routes resource exists and it has one port that is the default.
			expect := &pbmesh.ComputedRoutes{
				BoundReferences: []*pbresource.Reference{
					apiServiceRef,
				},
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						UsingDefaultConfig: true,
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								Rules: []*pbmesh.ComputedTCPRouteRule{{
									BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
										BackendTarget: backendName("api", "tcp", apiServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "tcp"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_TCP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "tcp", apiServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(apiServiceRef, "tcp", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, "", expect)
		})

		// Let the default http/http2/grpc routes get created.

		apiServiceData = &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{TargetPort: "http2", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
				{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			},
		}

		_ = rtest.Resource(pbcatalog.ServiceType, "api").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), apiServiceData).
			Write(suite.T(), suite.client)

		// also create the fooService so we can point to it.
		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{TargetPort: "http2", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
				{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			},
		}

		fooService := rtest.Resource(pbcatalog.ServiceType, "foo").
			WithTenancy(fooServiceRef.Tenancy).
			WithData(suite.T(), fooServiceData).
			Write(suite.T(), suite.client)

		testutil.RunStep(suite.T(), "default other routes", func(t *testing.T) {
			expect := &pbmesh.ComputedRoutes{
				BoundReferences: []*pbresource.Reference{
					apiServiceRef,
				},
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						UsingDefaultConfig: true,
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								Rules: []*pbmesh.ComputedTCPRouteRule{{
									BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
										BackendTarget: backendName("api", "tcp", apiServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "tcp"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_TCP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "tcp", apiServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(apiServiceRef, "tcp", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http": {
						UsingDefaultConfig: true,
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{{
									Matches: []*pbmesh.HTTPRouteMatch{{
										Path: &pbmesh.HTTPPathMatch{
											Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
											Value: "/",
										},
									}},
									BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
										BackendTarget: backendName("api", "http", apiServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "http", apiServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(apiServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http2": {
						UsingDefaultConfig: true,
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{{
									Matches: []*pbmesh.HTTPRouteMatch{{
										Path: &pbmesh.HTTPPathMatch{
											Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
											Value: "/",
										},
									}},
									BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
										BackendTarget: backendName("api", "http2", apiServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http2"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP2,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "http2", apiServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(apiServiceRef, "http2", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"grpc": {
						UsingDefaultConfig: true,
						Config: &pbmesh.ComputedPortRoutes_Grpc{
							Grpc: &pbmesh.ComputedGRPCRoute{
								Rules: []*pbmesh.ComputedGRPCRouteRule{{
									Matches: []*pbmesh.GRPCRouteMatch{{}},
									BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
										BackendTarget: backendName("api", "grpc", apiServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "grpc"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_GRPC,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "grpc", apiServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(apiServiceRef, "grpc", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, lastVersion, expect)
		})

		// Customize each route type.

		tcpRoute1 := &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "tcp"),
			},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}
		tcpRoute1ID := rtest.Resource(pbmesh.TCPRouteType, "api-tcp-route1").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), tcpRoute1).
			Write(suite.T(), suite.client).
			Id

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http"),
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http2"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}
		httpRoute1ID := rtest.Resource(pbmesh.HTTPRouteType, "api-http-route1").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), httpRoute1).
			Write(suite.T(), suite.client).
			Id

		grpcRoute1 := &pbmesh.GRPCRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "grpc"),
			},
			Rules: []*pbmesh.GRPCRouteRule{{
				BackendRefs: []*pbmesh.GRPCBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}
		grpcRoute1ID := rtest.Resource(pbmesh.GRPCRouteType, "api-grpc-route1").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), grpcRoute1).
			Write(suite.T(), suite.client).
			Id

		testutil.RunStep(suite.T(), "one of each", func(t *testing.T) {
			boundRefs := []*pbresource.Reference{
				fooServiceRef,
				apiServiceRef,
				resource.Reference(grpcRoute1ID, ""),
				resource.Reference(httpRoute1ID, ""),
				resource.Reference(tcpRoute1ID, ""),
			}
			sort.Slice(boundRefs, func(i, j int) bool {
				return resource.LessReference(boundRefs[i], boundRefs[j])
			})

			expect := &pbmesh.ComputedRoutes{
				BoundReferences: boundRefs,
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								Rules: []*pbmesh.ComputedTCPRouteRule{{
									BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
										BackendTarget: backendName("foo", "tcp", fooServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "tcp"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_TCP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "tcp", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "tcp", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"grpc": {
						Config: &pbmesh.ComputedPortRoutes_Grpc{
							Grpc: &pbmesh.ComputedGRPCRoute{
								Rules: []*pbmesh.ComputedGRPCRouteRule{
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: backendName("foo", "grpc", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "grpc"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_GRPC,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "grpc", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "grpc", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http2": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http2", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http2"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP2,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http2", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http2", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, lastVersion, expect)

			suite.client.WaitForStatusCondition(t, tcpRoute1ID, StatusKey, ConditionXRouteOK)
			suite.client.WaitForStatusCondition(t, httpRoute1ID, StatusKey, ConditionXRouteOK)
			suite.client.WaitForStatusCondition(t, grpcRoute1ID, StatusKey, ConditionXRouteOK)
		})

		// Add another route, with a bad mapping.

		tcpRoute2 := &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "tcp"),
			},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{{
					BackendRef: newBackendRef(barServiceRef, "", ""),
				}},
			}},
		}
		tcpRoute2ID := rtest.Resource(pbmesh.TCPRouteType, "api-tcp-route2").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), tcpRoute2).
			Write(suite.T(), suite.client).
			Id

		httpRoute2 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http"),
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http2"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				Matches: []*pbmesh.HTTPRouteMatch{{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/healthz",
					},
				}},
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(barServiceRef, "", ""),
				}},
			}},
		}
		httpRoute2ID := rtest.Resource(pbmesh.HTTPRouteType, "api-http-route2").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), httpRoute2).
			Write(suite.T(), suite.client).
			Id

		grpcRoute2 := &pbmesh.GRPCRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "grpc"),
			},
			Rules: []*pbmesh.GRPCRouteRule{{
				Matches: []*pbmesh.GRPCRouteMatch{{
					Method: &pbmesh.GRPCMethodMatch{
						Type:    pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
						Service: "billing",
						Method:  "charge",
					},
				}},
				BackendRefs: []*pbmesh.GRPCBackendRef{{
					BackendRef: newBackendRef(barServiceRef, "", ""),
				}},
			}},
		}
		grpcRoute2ID := rtest.Resource(pbmesh.GRPCRouteType, "api-grpc-route2").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), grpcRoute2).
			Write(suite.T(), suite.client).
			Id

		testutil.RunStep(suite.T(), "one good one bad route", func(t *testing.T) {
			boundRefs := []*pbresource.Reference{
				fooServiceRef,
				apiServiceRef,
				resource.Reference(grpcRoute1ID, ""),
				resource.Reference(grpcRoute2ID, ""),
				resource.Reference(httpRoute1ID, ""),
				resource.Reference(httpRoute2ID, ""),
				resource.Reference(tcpRoute1ID, ""),
				resource.Reference(tcpRoute2ID, ""),
			}
			sort.Slice(boundRefs, func(i, j int) bool {
				return resource.LessReference(boundRefs[i], boundRefs[j])
			})

			expect := &pbmesh.ComputedRoutes{
				BoundReferences: boundRefs,
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								Rules: []*pbmesh.ComputedTCPRouteRule{
									{
										BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
											BackendTarget: backendName("foo", "tcp", fooServiceRef.Tenancy),
										}},
									},
									{
										BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "tcp"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_TCP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "tcp", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "tcp", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: []*pbmesh.HTTPRouteMatch{{
											Path: &pbmesh.HTTPPathMatch{
												Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
												Value: "/healthz",
											},
										}},
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"grpc": {
						Config: &pbmesh.ComputedPortRoutes_Grpc{
							Grpc: &pbmesh.ComputedGRPCRoute{
								Rules: []*pbmesh.ComputedGRPCRouteRule{
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: backendName("foo", "grpc", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: []*pbmesh.GRPCRouteMatch{{
											Method: &pbmesh.GRPCMethodMatch{
												Type:    pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
												Service: "billing",
												Method:  "charge",
											},
										}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "grpc"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_GRPC,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "grpc", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "grpc", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http2": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: []*pbmesh.HTTPRouteMatch{{
											Path: &pbmesh.HTTPPathMatch{
												Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
												Value: "/healthz",
											},
										}},
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http2", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http2"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP2,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http2", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http2", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, lastVersion, expect)

			suite.client.WaitForStatusCondition(t, tcpRoute1ID, StatusKey, ConditionXRouteOK)
			suite.client.WaitForStatusCondition(t, httpRoute1ID, StatusKey, ConditionXRouteOK)
			suite.client.WaitForStatusCondition(t, grpcRoute1ID, StatusKey, ConditionXRouteOK)

			suite.client.WaitForStatusCondition(t, tcpRoute2ID, StatusKey,
				ConditionMissingBackendRef(newRef(pbcatalog.ServiceType, "bar", barServiceRef.Tenancy)))
			suite.client.WaitForStatusCondition(t, httpRoute2ID, StatusKey,
				ConditionMissingBackendRef(newRef(pbcatalog.ServiceType, "bar", barServiceRef.Tenancy)))
			suite.client.WaitForStatusCondition(t, grpcRoute2ID, StatusKey,
				ConditionMissingBackendRef(newRef(pbcatalog.ServiceType, "bar", barServiceRef.Tenancy)))
		})

		// Update the route2 routes to point to a real service, but overlap in
		// their parentrefs with existing ports tied to other xRoutes.
		//
		// tcp2 -> http1
		// http2 -> grpc1
		// grpc2 -> tcp1
		//
		// Also remove customization for the protocol http2.

		tcpRoute2 = &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http"),
			},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}
		rtest.ResourceID(tcpRoute2ID).
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), tcpRoute2).
			Write(suite.T(), suite.client)

		httpRoute2 = &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "grpc"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				Matches: []*pbmesh.HTTPRouteMatch{{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/healthz",
					},
				}},
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}
		rtest.ResourceID(httpRoute2ID).
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), httpRoute2).
			Write(suite.T(), suite.client)

		grpcRoute2 = &pbmesh.GRPCRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "tcp"),
			},
			Rules: []*pbmesh.GRPCRouteRule{{
				Matches: []*pbmesh.GRPCRouteMatch{{
					Method: &pbmesh.GRPCMethodMatch{
						Type:    pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
						Service: "billing",
						Method:  "charge",
					},
				}},
				BackendRefs: []*pbmesh.GRPCBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}
		rtest.ResourceID(grpcRoute2ID).
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), grpcRoute2).
			Write(suite.T(), suite.client)

		testutil.RunStep(suite.T(), "overlapping xRoutes generate conflicts", func(t *testing.T) {
			boundRefs := []*pbresource.Reference{
				apiServiceRef,
				fooServiceRef,
				resource.Reference(grpcRoute1ID, ""),
				resource.Reference(grpcRoute2ID, ""),
				resource.Reference(httpRoute1ID, ""),
				resource.Reference(httpRoute2ID, ""),
				resource.Reference(tcpRoute1ID, ""),
				resource.Reference(tcpRoute2ID, ""),
			}
			sort.Slice(boundRefs, func(i, j int) bool {
				return resource.LessReference(boundRefs[i], boundRefs[j])
			})

			expect := &pbmesh.ComputedRoutes{
				BoundReferences: boundRefs,
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								Rules: []*pbmesh.ComputedTCPRouteRule{{
									BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
										BackendTarget: backendName("foo", "tcp", fooServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "tcp"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_TCP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "tcp", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "tcp", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"grpc": {
						Config: &pbmesh.ComputedPortRoutes_Grpc{
							Grpc: &pbmesh.ComputedGRPCRoute{
								Rules: []*pbmesh.ComputedGRPCRouteRule{
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: backendName("foo", "grpc", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "grpc"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_GRPC,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "grpc", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "grpc", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http2": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http2", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http2"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP2,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http2", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http2", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, lastVersion, expect)

			suite.client.WaitForStatusCondition(t, tcpRoute1ID, StatusKey, ConditionXRouteOK)
			suite.client.WaitForStatusCondition(t, httpRoute1ID, StatusKey, ConditionXRouteOK)
			suite.client.WaitForStatusCondition(t, grpcRoute1ID, StatusKey, ConditionXRouteOK)

			suite.client.WaitForStatusCondition(t, tcpRoute2ID, StatusKey,
				ConditionConflictNotBoundToParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http", pbmesh.HTTPRouteType))
			suite.client.WaitForStatusCondition(t, httpRoute2ID, StatusKey,
				ConditionConflictNotBoundToParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "grpc", pbmesh.GRPCRouteType))
			suite.client.WaitForStatusCondition(t, grpcRoute2ID, StatusKey,
				ConditionConflictNotBoundToParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "tcp", pbmesh.TCPRouteType))
		})

		// - Delete the bad routes
		// - delete the original grpc route
		// - create a new grpc route with a later name so it loses the conflict
		//   battle, and do a wildcard port binding

		suite.client.MustDelete(suite.T(), tcpRoute2ID)
		suite.client.MustDelete(suite.T(), httpRoute2ID)
		suite.client.MustDelete(suite.T(), grpcRoute1ID)
		suite.client.MustDelete(suite.T(), grpcRoute2ID)

		suite.client.WaitForDeletion(suite.T(), tcpRoute2ID)
		suite.client.WaitForDeletion(suite.T(), httpRoute2ID)
		suite.client.WaitForDeletion(suite.T(), grpcRoute1ID)
		suite.client.WaitForDeletion(suite.T(), grpcRoute2ID)

		// Re-create with newarly the same data (wildcard port now) with a newer name.
		grpcRoute1 = &pbmesh.GRPCRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), ""),
			},
			Rules: []*pbmesh.GRPCRouteRule{{
				BackendRefs: []*pbmesh.GRPCBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}
		grpcRoute1ID = rtest.Resource(pbmesh.GRPCRouteType, "zzz-bad-route").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), grpcRoute1).
			Write(suite.T(), suite.client).
			Id

		testutil.RunStep(suite.T(), "overlapping xRoutes due to port wildcarding", func(t *testing.T) {
			boundRefs := []*pbresource.Reference{
				apiServiceRef,
				fooServiceRef,
				resource.Reference(grpcRoute1ID, ""),
				resource.Reference(httpRoute1ID, ""),
				resource.Reference(tcpRoute1ID, ""),
			}
			sort.Slice(boundRefs, func(i, j int) bool {
				return resource.LessReference(boundRefs[i], boundRefs[j])
			})

			expect := &pbmesh.ComputedRoutes{
				BoundReferences: boundRefs,
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								Rules: []*pbmesh.ComputedTCPRouteRule{{
									BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
										BackendTarget: backendName("foo", "tcp", fooServiceRef.Tenancy),
									}},
								}},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "tcp"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_TCP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "tcp", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "tcp", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"grpc": {
						Config: &pbmesh.ComputedPortRoutes_Grpc{
							Grpc: &pbmesh.ComputedGRPCRoute{
								Rules: []*pbmesh.ComputedGRPCRouteRule{
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: backendName("foo", "grpc", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "grpc"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_GRPC,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "grpc", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "grpc", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
					"http2": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http2", fooServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(apiServiceRef, "http2"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP2,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http2", fooServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(fooServiceRef, "http2", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			suite.client.WaitForStatusConditions(t, grpcRoute1ID, StatusKey,
				ConditionConflictNotBoundToParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http", pbmesh.HTTPRouteType),
				ConditionConflictNotBoundToParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "http2", pbmesh.HTTPRouteType),
				ConditionConflictNotBoundToParentRef(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy), "tcp", pbmesh.TCPRouteType))

			lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, "" /*no change*/, expect)

			suite.client.WaitForStatusCondition(t, tcpRoute1ID, StatusKey, ConditionXRouteOK)
			suite.client.WaitForStatusCondition(t, httpRoute1ID, StatusKey, ConditionXRouteOK)

		})

		// Remove the mesh port from api service.

		apiServiceData = &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				{TargetPort: "http2", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
				{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
			},
		}

		apiService := rtest.Resource(pbcatalog.ServiceType, "api").
			WithTenancy(apiServiceRef.Tenancy).
			WithData(suite.T(), apiServiceData).
			Write(suite.T(), suite.client)

		testutil.RunStep(suite.T(), "entire generated resource is deleted", func(t *testing.T) {
			suite.client.WaitForDeletion(t, computedRoutesID)

			suite.client.WaitForStatusCondition(t, tcpRoute1ID, StatusKey,
				ConditionParentRefOutsideMesh(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy)))
			suite.client.WaitForStatusCondition(t, httpRoute1ID, StatusKey,
				ConditionParentRefOutsideMesh(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy)))
			suite.client.WaitForStatusCondition(t, grpcRoute1ID, StatusKey,
				ConditionParentRefOutsideMesh(newRef(pbcatalog.ServiceType, "api", apiServiceRef.Tenancy)))
		})

		// Get down to just 2 ports for all relevant services.
		for _, ref := range []*pbresource.Reference{apiServiceRef, fooServiceRef, barServiceRef} {
			_ = rtest.Resource(pbcatalog.ServiceType, ref.Name).
				WithTenancy(ref.Tenancy).
				WithData(suite.T(), &pbcatalog.Service{
					Workloads: &pbcatalog.WorkloadSelector{
						Prefixes: []string{ref.Name + "-"},
					},
					Ports: []*pbcatalog.ServicePort{
						{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
						{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
					},
				}).
				Write(suite.T(), suite.client)
		}

		httpRoute1 = &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(fooServiceRef, "http"),
				newParentRef(barServiceRef, "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(apiServiceRef, "", ""),
				}},
			}},
		}
		httpRoute1ID = rtest.Resource(pbmesh.HTTPRouteType, "route1").
			WithTenancy(fooServiceRef.Tenancy).
			WithData(suite.T(), httpRoute1).
			Write(suite.T(), suite.client).
			Id

		var (
			fooLastVersion string
			barLastVersion string

			fooComputedRoutesID = rtest.Resource(pbmesh.ComputedRoutesType, "foo").
						WithTenancy(fooServiceRef.Tenancy).
						ID()
			barComputedRoutesID = rtest.Resource(pbmesh.ComputedRoutesType, "bar").
						WithTenancy(barServiceRef.Tenancy).
						ID()
		)

		testutil.RunStep(suite.T(), "create a route linked to two parents", func(t *testing.T) {
			boundRefsFoo := []*pbresource.Reference{
				apiServiceRef,
				fooServiceRef,
				resource.Reference(httpRoute1ID, ""),
			}
			sort.Slice(boundRefsFoo, func(i, j int) bool {
				return resource.LessReference(boundRefsFoo[i], boundRefsFoo[j])
			})

			boundRefsBar := []*pbresource.Reference{
				apiServiceRef,
				barServiceRef,
				resource.Reference(httpRoute1ID, ""),
			}
			sort.Slice(boundRefsFoo, func(i, j int) bool {
				return resource.LessReference(boundRefsFoo[i], boundRefsFoo[j])
			})
			sort.Slice(boundRefsBar, func(i, j int) bool {
				return resource.LessReference(boundRefsBar[i], boundRefsBar[j])
			})

			expectFoo := &pbmesh.ComputedRoutes{
				BoundReferences: boundRefsFoo,
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("api", "http", apiServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(fooServiceRef, "http"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "http", apiServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(apiServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}
			expectBar := &pbmesh.ComputedRoutes{
				BoundReferences: boundRefsBar,
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("api", "http", apiServiceRef.Tenancy),
										}},
									},
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
										}},
									},
								},
							},
						},
						ParentRef: newParentRef(barServiceRef, "http"),
						Protocol:  pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "http", apiServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(apiServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			fooLastVersion = requireNewComputedRoutesVersion(t, suite.client, fooComputedRoutesID, fooLastVersion, expectFoo)
			barLastVersion = requireNewComputedRoutesVersion(t, suite.client, barComputedRoutesID, barLastVersion, expectBar)

			suite.client.WaitForStatusCondition(t, httpRoute1ID, StatusKey, ConditionXRouteOK)
		})

		// Remove bar parent
		httpRoute1 = &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(fooServiceRef, "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(apiServiceRef, "", ""),
				}},
			}},
		}
		httpRoute1ID = rtest.Resource(pbmesh.HTTPRouteType, "route1").
			WithTenancy(fooServiceRef.Tenancy).
			WithData(suite.T(), httpRoute1).
			Write(suite.T(), suite.client).
			Id

		testutil.RunStep(suite.T(), "remove a parent ref and show that the old computed routes is reconciled one more time", func(t *testing.T) {
			expectBar := &pbmesh.ComputedRoutes{
				BoundReferences: []*pbresource.Reference{
					barServiceRef,
				},
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("bar", "http", barServiceRef.Tenancy),
										}},
									},
								},
							},
						},
						UsingDefaultConfig: true,
						ParentRef:          newParentRef(barServiceRef, "http"),
						Protocol:           pbcatalog.Protocol_PROTOCOL_HTTP,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("bar", "http", barServiceRef.Tenancy): {
								Type:              pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
								MeshPort:          "mesh",
								BackendRef:        newBackendRef(barServiceRef, "http", ""),
								DestinationConfig: defaultDestConfig(),
							},
						},
					},
				},
			}

			barLastVersion = requireNewComputedRoutesVersion(t, suite.client, barComputedRoutesID, barLastVersion, expectBar)

			suite.client.WaitForStatusCondition(t, httpRoute1ID, StatusKey, ConditionXRouteOK)

			resourcesToDelete := []*pbresource.ID{
				apiService.Id,
				fooService.Id,
				tcpRoute1ID,
				tcpRoute2ID,
				grpcRoute1ID,
				grpcRoute2ID,
				httpRoute1ID,
				httpRoute2ID,
			}

			for _, id := range resourcesToDelete {
				suite.client.MustDelete(t, id)
				suite.client.WaitForDeletion(t, id)
			}
		})
	})
}

func newParentRef(ref *pbresource.Reference, port string) *pbmesh.ParentReference {
	return &pbmesh.ParentReference{
		Ref:  ref,
		Port: port,
	}
}

func newBackendRef(ref *pbresource.Reference, port string, datacenter string) *pbmesh.BackendReference {
	return &pbmesh.BackendReference{
		Ref:        ref,
		Port:       port,
		Datacenter: datacenter,
	}
}

func requireNewComputedRoutesVersion(
	t *testing.T,
	client *rtest.Client,
	id *pbresource.ID,
	version string,
	expected *pbmesh.ComputedRoutes,
) string {
	t.Helper()

	var nextVersion string
	retry.Run(t, func(r *retry.R) {
		res := client.WaitForNewVersion(r, id, version)

		var mc pbmesh.ComputedRoutes
		require.NoError(r, res.Data.UnmarshalTo(&mc))
		prototest.AssertDeepEqual(r, expected, &mc)

		nextVersion = res.Version
	})
	return nextVersion
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func (suite *controllerSuite) runTestCaseWithTenancies(testFunc func(ref *testResourceRef)) {
	for _, mainServiceTenancy := range suite.tenancies {
		for _, otherServiceTenancy := range suite.tenancies {

			apiServiceRef := rtest.Resource(pbcatalog.ServiceType, "api").
				WithTenancy(mainServiceTenancy).
				Reference("")
			fooServiceRef := rtest.Resource(pbcatalog.ServiceType, "foo").
				WithTenancy(otherServiceTenancy).
				Reference("")
			barServiceRef := rtest.Resource(pbcatalog.ServiceType, "bar").
				WithTenancy(otherServiceTenancy).
				Reference("")

			suite.Run(suite.appendTenancyInfo(mainServiceTenancy), func() {
				testFunc(&testResourceRef{
					apiServiceRef: apiServiceRef,
					fooServiceRef: fooServiceRef,
					barServiceRef: barServiceRef,
				})
			})
		}
	}
}

func (suite *controllerSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}
