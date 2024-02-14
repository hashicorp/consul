// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/proxytarget"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	meshv2beta1 "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type apiGWProxyStateTemplateBuilder struct {
	workload    *types.DecodedWorkload
	dataFetcher *fetcher.Fetcher
	dc          string
	computed    *meshv2beta1.ComputedGatewayConfiguration
	logger      hclog.Logger
	trustDomain string
}

func NewAPIGWProxyStateTemplateBuilder(workload *types.DecodedWorkload, configuration *meshv2beta1.ComputedGatewayConfiguration, logger hclog.Logger, dataFetcher *fetcher.Fetcher, dc, trustDomain string) *apiGWProxyStateTemplateBuilder {
	return &apiGWProxyStateTemplateBuilder{
		workload:    workload,
		dataFetcher: dataFetcher,
		computed:    configuration,
		dc:          dc,
		logger:      logger,
		trustDomain: trustDomain,
	}
}

func (b *apiGWProxyStateTemplateBuilder) identity() *pbresource.Reference {
	return &pbresource.Reference{
		Name:    b.workload.Data.Identity,
		Tenancy: b.workload.Id.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}
}

func (b *apiGWProxyStateTemplateBuilder) defaultDC(dc string) string {
	if dc != b.dc {
		panic("cross datacenter service discovery clusters are not supported in v2")
	}
	return dc
}

func (b *apiGWProxyStateTemplateBuilder) clustersAndEndpoints() (map[string]*pbproxystate.Cluster, map[string]*pbproxystate.EndpointRef) {
	// we always add the null route cluster since we target it by default
	clusters := map[string]*pbproxystate.Cluster{
		nullRouteClusterName: &pbproxystate.Cluster{
			Name: nullRouteClusterName,
			Group: &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: &pbproxystate.EndpointGroup{
					Group: &pbproxystate.EndpointGroup_Static{
						Static: &pbproxystate.StaticEndpointGroup{
							Config: &pbproxystate.StaticEndpointGroupConfig{
								ConnectTimeout: durationpb.New(10 * time.Second),
							},
						},
					},
				},
			},
			Protocol: pbproxystate.Protocol_PROTOCOL_TCP,
		},
	}
	endpoints := map[string]*pbproxystate.EndpointRef{}

	for _, listener := range b.computed.ListenerConfigs {
		for _, config := range listener.HostnameConfigs {
			listenerClusters, listenerEndpoints := proxytarget.ClustersAndEndpoints(config.Routes, b.trustDomain, b.identity().Name, b.defaultDC)
			for name, cluster := range listenerClusters {
				clusters[name] = cluster
			}
			for name, endpoint := range listenerEndpoints {
				endpoints[name] = endpoint
			}
		}
	}

	return clusters, endpoints
}

func makeInboundListener(
	name string,
	port uint32,
	tls *pbproxystate.TLSParameters,
	certificate *pbresource.Reference,
	workload *pbcatalog.Workload,
) *pbproxystate.Listener {
	listener := &pbproxystate.Listener{
		Name:      name,
		Direction: pbproxystate.Direction_DIRECTION_INBOUND,
		DefaultRouter: &pbproxystate.Router{
			Destination: &pbproxystate.Router_L4{
				L4: &pbproxystate.L4Destination{
					Destination: &pbproxystate.L4Destination_Cluster{
						Cluster: &pbproxystate.DestinationCluster{
							Name: nullRouteClusterName,
						},
					},
				},
			},
		},
	}

	if tls != nil {
		listener.DefaultRouter.InboundTls = &pbproxystate.TransportSocket{
			TlsParameters: tls,
			ConnectionTls: &pbproxystate.TransportSocket_InboundNonMesh{
				InboundNonMesh: &pbproxystate.InboundNonMeshTLS{
					Identity: &pbproxystate.InboundNonMeshTLS_LeafKey{
						LeafKey: certificate.String(),
					},
				},
			},
		}
	}

	addresses := workload.GetAddresses()

	// If there are more than one address, use the first one in the list.
	var address string
	if len(addresses) > 0 {
		address = addresses[0].Host
	}

	listener.BindAddress = &pbproxystate.Listener_HostPort{
		HostPort: &pbproxystate.HostPortAddress{
			Host: address,
			Port: port,
		},
	}

	// Add TLS inspection capability to be able to parse ALPN and/or SNI information from inbound connections.
	listener.Capabilities = append(listener.Capabilities, pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION)

	return listener
}

func makeL4RouterForDirect(
	tls *pbproxystate.TLSParameters,
	certificate *pbresource.Reference,
	clusterName,
	hostname string,
) *pbproxystate.Router {
	// For explicit destinations, we have no filter chain match, and filters
	// are based on port protocol.
	router := &pbproxystate.Router{
		Match: &pbproxystate.Match{
			ServerNames: []string{hostname},
		},
	}

	if tls != nil {
		router.InboundTls = &pbproxystate.TransportSocket{
			TlsParameters: tls,
			ConnectionTls: &pbproxystate.TransportSocket_InboundNonMesh{
				InboundNonMesh: &pbproxystate.InboundNonMeshTLS{
					Identity: &pbproxystate.InboundNonMeshTLS_LeafKey{
						LeafKey: certificate.String(),
					},
				},
			},
		}
	}

	statPrefix := fmt.Sprintf("upstream.%s", clusterName)

	router.Destination = &pbproxystate.Router_L4{
		L4: &pbproxystate.L4Destination{
			Destination: &pbproxystate.L4Destination_Cluster{
				Cluster: &pbproxystate.DestinationCluster{
					Name: clusterName,
				},
			},
			StatPrefix: statPrefix,
		},
	}

	return router
}

func makeL4RouterForSplit(
	tls *pbproxystate.TLSParameters,
	certificate *pbresource.Reference,
	clusters []*pbproxystate.L4WeightedDestinationCluster,
	hostname string,
) *pbproxystate.Router {
	// For explicit destinations, we have no filter chain match, and filters
	// are based on port protocol.
	router := &pbproxystate.Router{
		Match: &pbproxystate.Match{
			ServerNames: []string{hostname},
		},
	}

	if tls != nil {
		router.InboundTls = &pbproxystate.TransportSocket{
			TlsParameters: tls,
			ConnectionTls: &pbproxystate.TransportSocket_InboundNonMesh{
				InboundNonMesh: &pbproxystate.InboundNonMeshTLS{
					Identity: &pbproxystate.InboundNonMeshTLS_LeafKey{
						LeafKey: certificate.String(),
					},
				},
			},
		}
	}

	statPrefix := "upstream."

	router.Destination = &pbproxystate.Router_L4{
		L4: &pbproxystate.L4Destination{
			Destination: &pbproxystate.L4Destination_WeightedClusters{
				WeightedClusters: &pbproxystate.L4WeightedClusterGroup{
					Clusters: clusters,
				},
			},
			StatPrefix: statPrefix,
		},
	}

	return router
}

func (b *apiGWProxyStateTemplateBuilder) routerForHostname(
	tls *pbproxystate.TLSParameters,
	certificate *pbresource.Reference,
	hostname string,
	routes *meshv2beta1.ComputedPortRoutes,
) *pbproxystate.Router {
	var (
		targets = routes.Targets
	)

	switch config := routes.Config.(type) {
	case *meshv2beta1.ComputedPortRoutes_Tcp:
		route := config.Tcp

		if len(route.Rules) != 1 {
			panic("not possible due to validation and computation")
		}

		// When not using RDS we must generate a cluster name to attach to
		// the filter chain. With RDS, cluster names get attached to the
		// dynamic routes instead.

		routeRule := route.Rules[0]

		switch len(routeRule.BackendRefs) {
		case 0:
			panic("not possible to have a tcp route rule with no backend refs")
		case 1:
			tcpBackendRef := routeRule.BackendRefs[0]
			backend := targets[tcpBackendRef.BackendTarget]

			clusterName := proxytarget.ClusterSNI(
				backend.BackendRef.Ref,
				b.defaultDC(backend.BackendRef.Datacenter),
				b.trustDomain,
			)

			return makeL4RouterForDirect(tls, certificate, clusterName, hostname)
		default:
			clusters := make([]*pbproxystate.L4WeightedDestinationCluster, 0, len(routeRule.BackendRefs))
			for _, tcpBackendRef := range routeRule.BackendRefs {
				backend := targets[tcpBackendRef.BackendTarget]

				clusterName := proxytarget.ClusterSNI(
					backend.BackendRef.Ref,
					b.defaultDC(backend.BackendRef.Datacenter),
					b.trustDomain,
				)

				clusters = append(clusters, &pbproxystate.L4WeightedDestinationCluster{
					Name:   clusterName,
					Weight: wrapperspb.UInt32(tcpBackendRef.Weight),
				})
			}

			return makeL4RouterForSplit(tls, certificate, clusters, hostname)
		}
	default:
		return nil
	}
}

func (b *apiGWProxyStateTemplateBuilder) listenersAndRoutes() ([]*pbproxystate.Listener, map[string]*pbproxystate.Route) {
	listeners := []*pbproxystate.Listener{}
	routes := map[string]*pbproxystate.Route{}

	for listenerName, listenerConfig := range b.computed.ListenerConfigs {
		listener := makeInboundListener(listenerName, listenerConfig.Port, listenerConfig.TlsParameters, listenerConfig.Certificate, b.workload.Data)
		for hostname, computedHostname := range listenerConfig.HostnameConfigs {
			computedRoute := computedHostname.Routes

			router := b.routerForHostname(listenerConfig.TlsParameters, computedHostname.Certificate, hostname, computedRoute)

			if router != nil {
				listener.Routers = append(listener.Routers, router)
			}
		}
		listeners = append(listeners, listener)
	}

	return listeners, routes
}

func (b *apiGWProxyStateTemplateBuilder) certificates() map[string]*pbproxystate.LeafCertificate {
	return make(map[string]*pbproxystate.LeafCertificate)
}

func (b *apiGWProxyStateTemplateBuilder) Build() *meshv2beta1.ProxyStateTemplate {
	clusters, endpoints := b.clustersAndEndpoints()
	listeners, routes := b.listenersAndRoutes()

	return &meshv2beta1.ProxyStateTemplate{
		ProxyState: &meshv2beta1.ProxyState{
			Identity:         b.identity(),
			Listeners:        listeners,
			Clusters:         clusters,
			Routes:           routes,
			LeafCertificates: b.certificates(),
		},
		RequiredEndpoints:        endpoints,
		RequiredLeafCertificates: make(map[string]*pbproxystate.LeafCertificateRef),
		RequiredTrustBundles:     make(map[string]*pbproxystate.TrustBundleRef),
	}
}
