// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestGenerateComputedRoutes(t *testing.T) {
	registry := resource.NewRegistry()
	types.Register(registry)
	catalog.RegisterTypes(registry)

	run := func(
		t *testing.T,
		related *loader.RelatedResources,
		expect []*ComputedRoutesResult,
		expectPending PendingStatuses,
	) {
		pending := make(PendingStatuses)

		got := GenerateComputedRoutes(related, pending)

		prototest.AssertElementsMatch[*ComputedRoutesResult](
			t, expect, got,
		)

		require.Len(t, pending, len(expectPending))
		if len(expectPending) > 0 {
			for rk, expectItem := range expectPending {
				gotItem, ok := pending[rk]
				require.True(t, ok, "missing expected pending status for %v", rk)
				prototest.AssertDeepEqual(t, expectItem, gotItem)
			}
			for rk, _ := range pending {
				_, ok := expectPending[rk]
				require.True(t, ok, "extra pending status for %v", rk)
			}
		}
	}

	newService := func(name string, data *pbcatalog.Service) *types.DecodedService {
		svc := rtest.Resource(catalog.ServiceType, name).
			WithData(t, data).Build()
		rtest.ValidateAndNormalize(t, registry, svc)
		return rtest.MustDecode[*pbcatalog.Service](t, svc)
	}

	newHTTPRoute := func(name string, data *pbmesh.HTTPRoute) *types.DecodedHTTPRoute {
		svc := rtest.Resource(types.HTTPRouteType, name).
			WithData(t, data).Build()
		rtest.ValidateAndNormalize(t, registry, svc)
		return rtest.MustDecode[*pbmesh.HTTPRoute](t, svc)
	}
	newGRPCRoute := func(name string, data *pbmesh.GRPCRoute) *types.DecodedGRPCRoute {
		svc := rtest.Resource(types.GRPCRouteType, name).
			WithData(t, data).Build()
		rtest.ValidateAndNormalize(t, registry, svc)
		return rtest.MustDecode[*pbmesh.GRPCRoute](t, svc)
	}
	newTCPRoute := func(name string, data *pbmesh.TCPRoute) *types.DecodedTCPRoute {
		svc := rtest.Resource(types.TCPRouteType, name).
			WithData(t, data).Build()
		rtest.ValidateAndNormalize(t, registry, svc)
		return rtest.MustDecode[*pbmesh.TCPRoute](t, svc)
	}

	newDestPolicy := func(name string, data *pbmesh.DestinationPolicy) *types.DecodedDestinationPolicy {
		policy := rtest.Resource(types.DestinationPolicyType, name).
			WithData(t, data).Build()
		rtest.ValidateAndNormalize(t, registry, policy)
		return rtest.MustDecode[*pbmesh.DestinationPolicy](t, policy)
	}

	newFailPolicy := func(name string, data *pbcatalog.FailoverPolicy) *types.DecodedFailoverPolicy {
		policy := rtest.Resource(catalog.FailoverPolicyType, name).
			WithData(t, data).Build()
		rtest.ValidateAndNormalize(t, registry, policy)
		return rtest.MustDecode[*pbcatalog.FailoverPolicy](t, policy)
	}

	backendName := func(name, port string) string {
		return fmt.Sprintf("catalog.v1alpha1.Service/default.local.default/%s?port=%s", name, port)
	}

	var (
		apiServiceID        = rtest.Resource(catalog.ServiceType, "api").ID()
		apiServiceRef       = resource.Reference(apiServiceID, "")
		apiComputedRoutesID = rtest.Resource(types.ComputedRoutesType, "api").ID()

		fooServiceID  = rtest.Resource(catalog.ServiceType, "foo").ID()
		fooServiceRef = resource.Reference(fooServiceID, "")

		barServiceID  = rtest.Resource(catalog.ServiceType, "bar").ID()
		barServiceRef = resource.Reference(barServiceID, "")
	)

	t.Run("none", func(t *testing.T) {
		run(t, loader.NewRelatedResources(), nil, nil)
	})

	t.Run("no aligned service", func(t *testing.T) {
		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID)
		expect := []*ComputedRoutesResult{
			{
				ID:   apiComputedRoutesID,
				Data: nil,
			},
		}
		run(t, related, expect, nil)
	})

	t.Run("aligned service not in mesh", func(t *testing.T) {
		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(newService("api", &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			}))
		expect := []*ComputedRoutesResult{{
			ID:   apiComputedRoutesID,
			Data: nil,
		}}
		run(t, related, expect, nil)
	})

	t.Run("tcp service with default route", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(newService("api", apiServiceData))

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								ParentRef: newParentRef(apiServiceRef, "tcp"),
								Rules: []*pbmesh.ComputedTCPRouteRule{{
									BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
										BackendTarget: backendName("api", "tcp"),
									}},
								}},
							},
						},
						UsingDefaultConfig: true,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "tcp"): {
								BackendRef: newBackendRef(apiServiceRef, "tcp", ""),
								Service:    apiServiceData,
							},
						},
					},
				},
			}},
		}
		run(t, related, expect, nil)
	})

	for protoName, protocol := range map[string]pbcatalog.Protocol{
		"http":  pbcatalog.Protocol_PROTOCOL_HTTP,
		"http2": pbcatalog.Protocol_PROTOCOL_HTTP2,
	} {
		t.Run(protoName+" service with default route", func(t *testing.T) {
			apiServiceData := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: protoName, Protocol: protocol},
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			}

			related := loader.NewRelatedResources().
				AddComputedRoutesIDs(apiComputedRoutesID).
				AddResources(newService("api", apiServiceData))

			expect := []*ComputedRoutesResult{{
				ID:      apiComputedRoutesID,
				OwnerID: apiServiceID,
				Data: &pbmesh.ComputedRoutes{
					PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
						protoName: {
							Config: &pbmesh.ComputedPortRoutes_Http{
								Http: &pbmesh.ComputedHTTPRoute{
									ParentRef: newParentRef(apiServiceRef, protoName),
									Rules: []*pbmesh.ComputedHTTPRouteRule{{
										Matches: []*pbmesh.HTTPRouteMatch{{
											Path: &pbmesh.HTTPPathMatch{
												Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
												Value: "/",
											},
										}},
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("api", protoName),
										}},
									}},
								},
							},
							UsingDefaultConfig: true,
							Targets: map[string]*pbmesh.BackendTargetDetails{
								backendName("api", protoName): {
									BackendRef: newBackendRef(apiServiceRef, protoName, ""),
									Service:    apiServiceData,
								},
							},
						},
					},
				},
			}}
			run(t, related, expect, nil)
		})
	}

	t.Run("grpc service with default route", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(newService("api", apiServiceData))

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"grpc": {
						Config: &pbmesh.ComputedPortRoutes_Grpc{
							Grpc: &pbmesh.ComputedGRPCRoute{
								ParentRef: newParentRef(apiServiceRef, "grpc"),
								Rules: []*pbmesh.ComputedGRPCRouteRule{{
									Matches: []*pbmesh.GRPCRouteMatch{{}},
									BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
										BackendTarget: backendName("api", "grpc"),
									}},
								}},
							},
						},
						UsingDefaultConfig: true,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "grpc"): {
								BackendRef: newBackendRef(apiServiceRef, "grpc", ""),
								Service:    apiServiceData,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("all ports with a mix of routes", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
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

		tcpRoute1 := &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "tcp"),
			},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
				newParentRef(newRef(catalog.ServiceType, "api"), "http2"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		grpcRoute1 := &pbmesh.GRPCRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "grpc"),
			},
			Rules: []*pbmesh.GRPCRouteRule{{
				BackendRefs: []*pbmesh.GRPCBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newTCPRoute("api-tcp-route1", tcpRoute1),
				newHTTPRoute("api-http-route1", httpRoute1),
				newGRPCRoute("api-grpc-route1", grpcRoute1),
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp": {
						Config: &pbmesh.ComputedPortRoutes_Tcp{
							Tcp: &pbmesh.ComputedTCPRoute{
								ParentRef: newParentRef(apiServiceRef, "tcp"),
								Rules: []*pbmesh.ComputedTCPRouteRule{{
									BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
										BackendTarget: backendName("foo", "tcp"),
									}},
								}},
							},
						},
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "tcp"): {
								BackendRef: newBackendRef(fooServiceRef, "tcp", ""),
								Service:    fooServiceData,
							},
						},
					},
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http"): {
								BackendRef: newBackendRef(fooServiceRef, "http", ""),
								Service:    fooServiceData,
							},
						},
					},
					"http2": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http2"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http2"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http2"): {
								BackendRef: newBackendRef(fooServiceRef, "http2", ""),
								Service:    fooServiceData,
							},
						},
					},
					"grpc": {
						Config: &pbmesh.ComputedPortRoutes_Grpc{
							Grpc: &pbmesh.ComputedGRPCRoute{
								ParentRef: newParentRef(apiServiceRef, "grpc"),
								Rules: []*pbmesh.ComputedGRPCRouteRule{
									{
										Matches: []*pbmesh.GRPCRouteMatch{{}},
										BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
											BackendTarget: backendName("foo", "grpc"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "grpc"): {
								BackendRef: newBackendRef(fooServiceRef, "grpc", ""),
								Service:    fooServiceData,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("all ports with a wildcard route only bypassing the protocol", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
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

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), ""),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newHTTPRoute("api-http-route1", httpRoute1),
			)

		chunk := func(portName string) *pbmesh.ComputedPortRoutes {
			return &pbmesh.ComputedPortRoutes{
				Config: &pbmesh.ComputedPortRoutes_Http{
					Http: &pbmesh.ComputedHTTPRoute{
						ParentRef: newParentRef(apiServiceRef, portName),
						Rules: []*pbmesh.ComputedHTTPRouteRule{
							{
								Matches: defaultHTTPRouteMatches(),
								BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
									BackendTarget: backendName("foo", portName),
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
				Targets: map[string]*pbmesh.BackendTargetDetails{
					backendName("foo", portName): {
						BackendRef: newBackendRef(fooServiceRef, portName, ""),
						Service:    fooServiceData,
					},
				},
			}
		}

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"tcp":   chunk("tcp"),
					"http":  chunk("http"),
					"http2": chunk("http2"),
					"grpc":  chunk("grpc"),
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("stale-weird: stale mapper causes visit of irrelevant xRoute", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), ""),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		apiSvc := newService("api", apiServiceData)
		fooSvc := newService("foo", fooServiceData)
		apiHTTPRoute1 := newHTTPRoute("api-http-route1", httpRoute1)

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID). // deliberately skip adding 'foo' here to exercise the bug
			AddResources(apiSvc, fooSvc, apiHTTPRoute1)

		// Update this after the fact, but don't update the inner indexing in
		// the 'related' struct.
		{
			httpRoute1.ParentRefs[0] = newParentRef(newRef(catalog.ServiceType, "foo"), "")
			apiHTTPRoute1.Data = httpRoute1

			anyData, err := anypb.New(httpRoute1)
			require.NoError(t, err)
			apiHTTPRoute1.Resource.Data = anyData
		}

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{{
									Matches: []*pbmesh.HTTPRouteMatch{{
										Path: &pbmesh.HTTPPathMatch{
											Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
											Value: "/",
										},
									}},
									BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
										BackendTarget: backendName("api", "http"),
									}},
								}},
							},
						},
						UsingDefaultConfig: true,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "http"): {
								BackendRef: newBackendRef(apiServiceRef, "http", ""),
								Service:    apiServiceData,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("stale-weird: parent ref uses invalid port", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				// Using bad parent port (www).
				newParentRef(newRef(catalog.ServiceType, "api"), "www"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "http", ""),
				}},
			}},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newHTTPRoute("api-http-route1", httpRoute1),
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{{
									Matches: []*pbmesh.HTTPRouteMatch{{
										Path: &pbmesh.HTTPPathMatch{
											Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
											Value: "/",
										},
									}},
									BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
										BackendTarget: backendName("api", "http"),
									}},
								}},
							},
						},
						UsingDefaultConfig: true,
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("api", "http"): {
								BackendRef: newBackendRef(apiServiceRef, "http", ""),
								Service:    apiServiceData,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("overlapping xRoutes", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRouteData := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				// Using bad parent port (www).
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "http", ""),
				}},
			}},
		}
		httpRoute := newHTTPRoute("api-http-route", httpRouteData)

		tcpRouteData := &pbmesh.TCPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				// Using bad parent port (www).
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.TCPRouteRule{{
				BackendRefs: []*pbmesh.TCPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "http", ""),
				}},
			}},
		}
		tcpRoute := newTCPRoute("api-tcp-route", tcpRouteData)
		// Force them to have the same generation, so that we fall back on
		// lexicographic sort on the names as tiebreaker.
		//
		// api-http-route < api-tcp-route
		tcpRoute.Resource.Generation = httpRoute.Resource.Generation

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				httpRoute,
				tcpRoute,
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http"): {
								BackendRef: newBackendRef(fooServiceRef, "http", ""),
								Service:    fooServiceData,
							},
						},
					},
				},
			},
		}}

		expectPending := make(PendingStatuses)
		expectPending[resource.NewReferenceKey(tcpRoute.Resource.Id)] = &PendingResourceStatusUpdate{
			ID:         tcpRoute.Resource.Id,
			Generation: tcpRoute.Resource.Generation,
			NewConditions: []*pbresource.Condition{
				ConditionConflictNotBoundToParentRef(
					apiServiceRef,
					"http",
					types.HTTPRouteType,
				),
			},
		}

		run(t, related, expect, expectPending)
	})

	t.Run("two http routes", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		barServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"bar-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				Matches: []*pbmesh.HTTPRouteMatch{{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/gir",
					},
				}},
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		httpRoute2 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				Matches: []*pbmesh.HTTPRouteMatch{{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/zim",
					},
				}},
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(barServiceRef, "", ""),
				}},
			}},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newService("bar", barServiceData),
				newHTTPRoute("api-http-route1", httpRoute1),
				newHTTPRoute("api-http-route2", httpRoute2),
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: []*pbmesh.HTTPRouteMatch{{
											Path: &pbmesh.HTTPPathMatch{
												Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
												Value: "/gir",
											},
										}},
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http"),
										}},
									},
									{
										Matches: []*pbmesh.HTTPRouteMatch{{
											Path: &pbmesh.HTTPPathMatch{
												Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
												Value: "/zim",
											},
										}},
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("bar", "http"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http"): {
								BackendRef: newBackendRef(fooServiceRef, "http", ""),
								Service:    fooServiceData,
							},
							backendName("bar", "http"): {
								BackendRef: newBackendRef(barServiceRef, "http", ""),
								Service:    barServiceData,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("http route with empty match path", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				Matches: []*pbmesh.HTTPRouteMatch{{
					Path: nil,
				}},
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newHTTPRoute("api-http-route1", httpRoute1),
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http"): {
								BackendRef: newBackendRef(fooServiceRef, "http", ""),
								Service:    fooServiceData,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("stale-weird: destination with no service", func(t *testing.T) {
		t.Run("http", func(t *testing.T) {
			apiServiceData := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
				},
			}

			httpRoute1 := &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(newRef(catalog.ServiceType, "api"), "http"),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(fooServiceRef, "", ""),
					}},
				}},
			}

			related := loader.NewRelatedResources().
				AddComputedRoutesIDs(apiComputedRoutesID).
				AddResources(
					newService("api", apiServiceData),
					newHTTPRoute("api-http-route1", httpRoute1),
				)

			expect := []*ComputedRoutesResult{{
				ID:      apiComputedRoutesID,
				OwnerID: apiServiceID,
				Data: &pbmesh.ComputedRoutes{
					PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
						"http": {
							Config: &pbmesh.ComputedPortRoutes_Http{
								Http: &pbmesh.ComputedHTTPRoute{
									ParentRef: newParentRef(apiServiceRef, "http"),
									Rules: []*pbmesh.ComputedHTTPRouteRule{
										{
											Matches: defaultHTTPRouteMatches(),
											BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
												BackendTarget: types.NullRouteBackend,
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
						},
					},
				},
			}}
			run(t, related, expect, nil)
		})
		t.Run("grpc", func(t *testing.T) {
			apiServiceData := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
			}

			grpcRoute1 := &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(newRef(catalog.ServiceType, "api"), "grpc"),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(fooServiceRef, "", ""),
					}},
				}},
			}

			related := loader.NewRelatedResources().
				AddComputedRoutesIDs(apiComputedRoutesID).
				AddResources(
					newService("api", apiServiceData),
					newGRPCRoute("api-grpc-route1", grpcRoute1),
				)

			expect := []*ComputedRoutesResult{{
				ID:      apiComputedRoutesID,
				OwnerID: apiServiceID,
				Data: &pbmesh.ComputedRoutes{
					PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
						"grpc": {
							Config: &pbmesh.ComputedPortRoutes_Grpc{
								Grpc: &pbmesh.ComputedGRPCRoute{
									ParentRef: newParentRef(apiServiceRef, "grpc"),
									Rules: []*pbmesh.ComputedGRPCRouteRule{
										{
											Matches: defaultGRPCRouteMatches(),
											BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
												BackendTarget: types.NullRouteBackend,
											}},
										},
										{
											Matches: defaultGRPCRouteMatches(),
											BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
												BackendTarget: types.NullRouteBackend,
											}},
										},
									},
								},
							},
						},
					},
				},
			}}
			run(t, related, expect, nil)
		})
		t.Run("tcp", func(t *testing.T) {
			apiServiceData := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{"api-"},
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			}

			tcpRoute1 := &pbmesh.TCPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(newRef(catalog.ServiceType, "api"), "tcp"),
				},
				Rules: []*pbmesh.TCPRouteRule{{
					BackendRefs: []*pbmesh.TCPBackendRef{{
						BackendRef: newBackendRef(fooServiceRef, "", ""),
					}},
				}},
			}

			related := loader.NewRelatedResources().
				AddComputedRoutesIDs(apiComputedRoutesID).
				AddResources(
					newService("api", apiServiceData),
					newTCPRoute("api-tcp-route1", tcpRoute1),
				)

			expect := []*ComputedRoutesResult{{
				ID:      apiComputedRoutesID,
				OwnerID: apiServiceID,
				Data: &pbmesh.ComputedRoutes{
					PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
						"tcp": {
							Config: &pbmesh.ComputedPortRoutes_Tcp{
								Tcp: &pbmesh.ComputedTCPRoute{
									ParentRef: newParentRef(apiServiceRef, "tcp"),
									Rules: []*pbmesh.ComputedTCPRouteRule{
										{
											BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
												BackendTarget: types.NullRouteBackend,
											}},
										},
									},
								},
							},
						},
					},
				},
			}}
			run(t, related, expect, nil)
		})
	})

	t.Run("stale-weird: http destination with service not in mesh", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newHTTPRoute("api-http-route1", httpRoute1),
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: types.NullRouteBackend,
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
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("http route with dest policy", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				Matches: []*pbmesh.HTTPRouteMatch{{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/",
					},
				}},
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		destPolicy := &pbmesh.DestinationPolicy{
			PortConfigs: map[string]*pbmesh.DestinationConfig{
				"http": {
					ConnectTimeout: durationpb.New(55 * time.Second),
				},
			},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newHTTPRoute("api-http-route1", httpRoute1),
				newDestPolicy("foo", destPolicy),
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http"): {
								BackendRef:        newBackendRef(fooServiceRef, "http", ""),
								Service:           fooServiceData,
								DestinationPolicy: destPolicy,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})

	t.Run("http route with failover policy", func(t *testing.T) {
		apiServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		fooServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		barServiceData := &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"bar-"},
			},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}

		httpRoute1 := &pbmesh.HTTPRoute{
			ParentRefs: []*pbmesh.ParentReference{
				newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			},
			Rules: []*pbmesh.HTTPRouteRule{{
				Matches: []*pbmesh.HTTPRouteMatch{{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/",
					},
				}},
				BackendRefs: []*pbmesh.HTTPBackendRef{{
					BackendRef: newBackendRef(fooServiceRef, "", ""),
				}},
			}},
		}

		failoverPolicy := &pbcatalog.FailoverPolicy{
			Config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{{
					Ref: barServiceRef,
				}},
			},
		}
		simplifiedFailoverPolicy := &pbcatalog.FailoverPolicy{
			PortConfigs: map[string]*pbcatalog.FailoverConfig{
				"http": &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{{
						Ref:  barServiceRef,
						Port: "http",
					}},
				},
			},
		}

		related := loader.NewRelatedResources().
			AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
				newService("api", apiServiceData),
				newService("foo", fooServiceData),
				newService("bar", barServiceData),
				newHTTPRoute("api-http-route1", httpRoute1),
				newFailPolicy("foo", failoverPolicy),
			)

		expect := []*ComputedRoutesResult{{
			ID:      apiComputedRoutesID,
			OwnerID: apiServiceID,
			Data: &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"http": {
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{
								ParentRef: newParentRef(apiServiceRef, "http"),
								Rules: []*pbmesh.ComputedHTTPRouteRule{
									{
										Matches: defaultHTTPRouteMatches(),
										BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
											BackendTarget: backendName("foo", "http"),
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
						Targets: map[string]*pbmesh.BackendTargetDetails{
							backendName("foo", "http"): {
								BackendRef:     newBackendRef(fooServiceRef, "http", ""),
								Service:        fooServiceData,
								FailoverPolicy: simplifiedFailoverPolicy,
							},
						},
					},
				},
			},
		}}
		run(t, related, expect, nil)
	})
}
