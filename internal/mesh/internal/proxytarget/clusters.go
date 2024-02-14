package proxytarget

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	nullRouteClusterName = "null_route_cluster"
)

func nullRouteCluster() *pbproxystate.Cluster {
	return &pbproxystate.Cluster{
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
	}
}

func ClusterSNI(serviceRef *pbresource.Reference, datacenter, trustDomain string) string {
	return connect.ServiceSNI(serviceRef.Name,
		"",
		serviceRef.Tenancy.Namespace,
		serviceRef.Tenancy.Partition,
		datacenter,
		trustDomain)
}

func newClusterEndpointGroup(
	clusterName string,
	sni string,
	portName string,
	trustDomain string,
	identityKey string,
	destinationIdentities []*pbresource.Reference,
	connectTimeout *durationpb.Duration,
	loadBalancer *pbmesh.LoadBalancer,
) *pbproxystate.EndpointGroup {
	var spiffeIDs []string
	for _, identity := range destinationIdentities {
		spiffeIDs = append(spiffeIDs, connect.SpiffeIDFromIdentityRef(trustDomain, identity))
	}

	// TODO(v2): DestinationPolicy: circuit breakers, outlier detection

	// TODO(v2): if http2/grpc then set http2protocol options

	degConfig := &pbproxystate.DynamicEndpointGroupConfig{
		DisablePanicThreshold: true,
		ConnectTimeout:        connectTimeout,
	}

	if loadBalancer != nil {
		// enumcover:pbmesh.LoadBalancerPolicy
		switch loadBalancer.Policy {
		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RANDOM:
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_Random{}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_ROUND_ROBIN:
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_RoundRobin{}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST:
			var choiceCount uint32
			cfg, ok := loadBalancer.Config.(*pbmesh.LoadBalancer_LeastRequestConfig)
			if ok {
				choiceCount = cfg.LeastRequestConfig.GetChoiceCount()
			}
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_LeastRequest{
				LeastRequest: &pbproxystate.LBPolicyLeastRequest{
					ChoiceCount: wrapperspb.UInt32(choiceCount),
				},
			}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV:
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_Maglev{}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH:
			policy := &pbproxystate.DynamicEndpointGroupConfig_RingHash{}

			cfg, ok := loadBalancer.Config.(*pbmesh.LoadBalancer_RingHashConfig)
			if ok {
				policy.RingHash = &pbproxystate.LBPolicyRingHash{
					MinimumRingSize: wrapperspb.UInt64(cfg.RingHashConfig.MinimumRingSize),
					MaximumRingSize: wrapperspb.UInt64(cfg.RingHashConfig.MaximumRingSize),
				}
			}

			degConfig.LbPolicy = policy

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_UNSPECIFIED:
			// fallthrough to default
		default:
			// do nothing
		}
	}

	return &pbproxystate.EndpointGroup{
		Name: clusterName,
		Group: &pbproxystate.EndpointGroup_Dynamic{
			Dynamic: &pbproxystate.DynamicEndpointGroup{
				Config: degConfig,
				OutboundTls: &pbproxystate.TransportSocket{
					ConnectionTls: &pbproxystate.TransportSocket_OutboundMesh{
						OutboundMesh: &pbproxystate.OutboundMeshMTLS{
							IdentityKey: identityKey,
							ValidationContext: &pbproxystate.MeshOutboundValidationContext{
								SpiffeIds:              spiffeIDs,
								TrustBundlePeerNameKey: resource.DefaultPeerName,
							},
							Sni: sni,
						},
					},
					AlpnProtocols: []string{GetAlpnProtocolFromPortName(portName)},
				},
			},
		},
	}
}

func ClustersAndEndpoints(routes *pbmesh.ComputedPortRoutes, trustDomain, identityKey string, defaultDC func(dc string) string) (map[string]*pbproxystate.Cluster, map[string]*pbproxystate.EndpointRef) {
	clusters := map[string]*pbproxystate.Cluster{}
	endpoints := map[string]*pbproxystate.EndpointRef{}

	targets := routes.Targets
	effectiveProtocol := routes.Protocol

	switch routeConfig := routes.Config.(type) {
	case *pbmesh.ComputedPortRoutes_Http:
		for _, rule := range routeConfig.Http.Rules {
			for _, backendRef := range rule.BackendRefs {
				if backendRef.BackendTarget == types.NullRouteBackend {
					clusters[nullRouteClusterName] = nullRouteCluster()
				}
			}
		}
	case *pbmesh.ComputedPortRoutes_Grpc:
		for _, rule := range routeConfig.Grpc.Rules {
			for _, backendRef := range rule.BackendRefs {
				if backendRef.BackendTarget == types.NullRouteBackend {
					clusters[nullRouteClusterName] = nullRouteCluster()
				}
			}
		}
	case *pbmesh.ComputedPortRoutes_Tcp:
		for _, rule := range routeConfig.Tcp.Rules {
			for _, backendRef := range rule.BackendRefs {
				if backendRef.BackendTarget == types.NullRouteBackend {
					clusters[nullRouteClusterName] = nullRouteCluster()
				}
			}
		}
	default:
		return clusters, endpoints
	}

	for _, details := range targets {
		// NOTE: we only emit clusters for DIRECT targets here.
		if details.Type != pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT {
			continue
		}

		connectTimeout := details.DestinationConfig.ConnectTimeout
		loadBalancer := details.DestinationConfig.LoadBalancer

		// NOTE: we collect both DIRECT and INDIRECT target information here.
		portName := details.BackendRef.Port
		sni := ClusterSNI(
			details.BackendRef.Ref,
			defaultDC(details.BackendRef.Datacenter),
			trustDomain,
		)
		clusterName := fmt.Sprintf("%s.%s", portName, sni)

		egName := ""
		if details.FailoverConfig != nil {
			egName = fmt.Sprintf("%s%d~%s", xdscommon.FailoverClusterNamePrefix, 0, clusterName)
		}
		egBase := newClusterEndpointGroup(egName, sni, portName, trustDomain, identityKey, details.IdentityRefs, connectTimeout, loadBalancer)

		var endpointGroups []*pbproxystate.EndpointGroup

		// Original target is the first (or only) target.
		endpointGroups = append(endpointGroups, egBase)
		endpoints[clusterName] = details.ServiceEndpointsRef

		if details.FailoverConfig != nil {
			failover := details.FailoverConfig
			// TODO(v2): handle other forms of failover (regions/locality/etc)

			for i, dest := range failover.Destinations {
				if dest.BackendTarget == types.NullRouteBackend {
					continue // not possible
				}
				destDetails, ok := targets[dest.BackendTarget]
				if !ok {
					continue // not possible
				}

				destConnectTimeout := destDetails.DestinationConfig.ConnectTimeout
				destLoadBalancer := destDetails.DestinationConfig.LoadBalancer

				destPortName := destDetails.BackendRef.Port

				destSNI := ClusterSNI(
					destDetails.BackendRef.Ref,
					defaultDC(destDetails.BackendRef.Datacenter),
					trustDomain,
				)

				// index 0 was already given to non-fail original
				failoverGroupIndex := i + 1
				destClusterName := fmt.Sprintf("%s%d~%s", xdscommon.FailoverClusterNamePrefix, failoverGroupIndex, clusterName)

				egDest := newClusterEndpointGroup(destClusterName, destSNI, destPortName, trustDomain, identityKey, destDetails.IdentityRefs, destConnectTimeout, destLoadBalancer)

				endpointGroups = append(endpointGroups, egDest)
				endpoints[destClusterName] = destDetails.ServiceEndpointsRef
			}
		}

		cluster := &pbproxystate.Cluster{
			Name:        clusterName,
			AltStatName: clusterName,
			Protocol:    pbproxystate.Protocol(effectiveProtocol),
		}
		switch len(endpointGroups) {
		case 0:
			panic("no endpoint groups provided")
		case 1:
			cluster.Group = &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: endpointGroups[0],
			}
		default:
			cluster.Group = &pbproxystate.Cluster_FailoverGroup{
				FailoverGroup: &pbproxystate.FailoverGroup{
					EndpointGroups: endpointGroups,
					Config: &pbproxystate.FailoverGroupConfig{
						UseAltStatName: true,
						ConnectTimeout: connectTimeout,
					},
				},
			}
		}

		clusters[cluster.Name] = cluster
	}

	return clusters, endpoints
}
