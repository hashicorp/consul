// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestGenerateComputedRoutes(t *testing.T) {
	registry := resource.NewRegistry()
	types.Register(registry)
	catalog.RegisterTypes(registry)

	run := func(t *testing.T, related *loader.RelatedResources, expect []*ComputedRoutesResult, expectPending int) {
		pending := make(PendingStatuses)

		got := GenerateComputedRoutes(related, pending)
		require.Len(t, pending, expectPending)

		prototest.AssertElementsMatch[*ComputedRoutesResult](
			t, expect, got,
		)
	}

	newService := func(name string, data *pbcatalog.Service) *types.DecodedService {
		svc := rtest.Resource(catalog.ServiceType, name).
			WithData(t, data).Build()
		rtest.ValidateAndNormalize(t, registry, svc)
		return rtest.MustDecode[*pbcatalog.Service](t, svc)
	}

	backendName := func(name, port string) string {
		return fmt.Sprintf("catalog.v1alpha1.Service/default.local.default/%s?port=%s", name, port)
	}

	var (
		apiServiceID        = rtest.Resource(catalog.ServiceType, "api").ID()
		apiServiceRef       = resource.Reference(apiServiceID, "")
		apiComputedRoutesID = rtest.Resource(types.ComputedRoutesType, "api").ID()
	)

	t.Run("none", func(t *testing.T) {
		run(t, loader.NewRelatedResources(), nil, 0)
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
		run(t, related, expect, 0)
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
		expect := []*ComputedRoutesResult{
			{
				ID:   apiComputedRoutesID,
				Data: nil,
			},
		}
		run(t, related, expect, 0)
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

		expect := []*ComputedRoutesResult{
			{
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
				},
			},
		}
		run(t, related, expect, 0)
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

			expect := []*ComputedRoutesResult{
				{
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
				},
			}
			run(t, related, expect, 0)
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

		expect := []*ComputedRoutesResult{
			{
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
			},
		}
		run(t, related, expect, 0)
	})
}
