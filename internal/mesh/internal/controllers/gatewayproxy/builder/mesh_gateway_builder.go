// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/meshgateways"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	meshv2beta1 "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	nullRouteClusterName = "null_route_cluster"
)

type meshGWProxyStateTemplateBuilder struct {
	workload         *types.DecodedWorkload
	dataFetcher      *fetcher.Fetcher
	dc               string
	exportedServices []*pbmulticluster.ComputedExportedService
	logger           hclog.Logger
	trustDomain      string
	remoteGatewayIDs []*pbresource.ID
}

func NewMeshGWProxyStateTemplateBuilder(workload *types.DecodedWorkload, exportedServices []*pbmulticluster.ComputedExportedService, logger hclog.Logger, dataFetcher *fetcher.Fetcher, dc, trustDomain string, remoteGatewayIDs []*pbresource.ID) *meshGWProxyStateTemplateBuilder {
	return &meshGWProxyStateTemplateBuilder{
		workload:         workload,
		dataFetcher:      dataFetcher,
		dc:               dc,
		exportedServices: exportedServices,
		logger:           logger,
		trustDomain:      trustDomain,
		remoteGatewayIDs: remoteGatewayIDs,
	}
}

func (b *meshGWProxyStateTemplateBuilder) identity() *pbresource.Reference {
	return &pbresource.Reference{
		Name:    b.workload.Data.Identity,
		Tenancy: b.workload.Id.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}
}

func (b *meshGWProxyStateTemplateBuilder) listeners() []*pbproxystate.Listener {
	var listeners []*pbproxystate.Listener
	var address *pbcatalog.WorkloadAddress

	// TODO: NET-7260 we think there should only ever be a single address for a gateway,
	// need to validate this
	if len(b.workload.Data.Addresses) > 0 {
		address = b.workload.Data.Addresses[0]
	}

	// if the address defines no ports we assume the intention is to bind to all
	// ports on the workload
	if len(address.Ports) == 0 {
		for portName, workloadPort := range b.workload.Data.Ports {
			switch portName {
			case meshgateways.LANPortName:
				listeners = append(listeners, b.meshListener(address, workloadPort.Port))
			case meshgateways.WANPortName:
				listeners = append(listeners, b.wanListener(address, workloadPort.Port))
			default:
				b.logger.Warn("encountered unexpected port on mesh gateway workload", "port", portName)
			}
		}
		return listeners
	}

	for _, portName := range address.Ports {
		workloadPort, ok := b.workload.Data.Ports[portName]
		if !ok {
			b.logger.Trace("port does not exist for workload", "port name", portName)
			continue
		}

		switch portName {
		case meshgateways.LANPortName:
			listeners = append(listeners, b.meshListener(address, workloadPort.Port))
		case meshgateways.WANPortName:
			listeners = append(listeners, b.wanListener(address, workloadPort.Port))
		default:
			b.logger.Warn("encountered unexpected port on mesh gateway workload", "port", portName)
		}
	}

	return listeners
}

// meshListener constructs a pbproxystate.Listener that receives outgoing
// traffic from the local partition where the mesh gateway mode is "local". This
// traffic will be sent to a mesh gateway in a remote partition.
func (b *meshGWProxyStateTemplateBuilder) meshListener(address *pbcatalog.WorkloadAddress, port uint32) *pbproxystate.Listener {
	return b.listener("mesh_listener", address, port, pbproxystate.Direction_DIRECTION_OUTBOUND, b.meshRouters())
}

// wanListener constructs a pbproxystate.Listener that receives incoming
// traffic from the public internet, either from a mesh gateway in a remote partition
// where the mesh gateway mode is "local" or from a service in a remote partition
// where the mesh gateway mode is "remote".
func (b *meshGWProxyStateTemplateBuilder) wanListener(address *pbcatalog.WorkloadAddress, port uint32) *pbproxystate.Listener {
	return b.listener("wan_listener", address, port, pbproxystate.Direction_DIRECTION_INBOUND, b.wanRouters())
}

func (b *meshGWProxyStateTemplateBuilder) listener(name string, address *pbcatalog.WorkloadAddress, port uint32, direction pbproxystate.Direction, routers []*pbproxystate.Router) *pbproxystate.Listener {
	return &pbproxystate.Listener{
		Name:      name,
		Direction: direction,
		BindAddress: &pbproxystate.Listener_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: address.Host,
				Port: port,
			},
		},
		Capabilities: []pbproxystate.Capability{
			pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION,
		},
		DefaultRouter: &pbproxystate.Router{
			Destination: &pbproxystate.Router_L4{
				L4: &pbproxystate.L4Destination{
					Destination: &pbproxystate.L4Destination_Cluster{
						Cluster: &pbproxystate.DestinationCluster{
							Name: nullRouteClusterName,
						},
					},
					StatPrefix: "prefix",
				},
			},
		},
		Routers: routers,
	}
}

// meshRouters loops through the list of mesh gateways in other partitions and generates
// a pbproxystate.Router matching the partition + datacenter of the SNI to the target
// cluster. Traffic flowing through this router originates in the local partition where
// the mesh gateway mode is "local".
func (b *meshGWProxyStateTemplateBuilder) meshRouters() []*pbproxystate.Router {
	var routers []*pbproxystate.Router

	for _, remoteGatewayID := range b.remoteGatewayIDs {
		serviceID := resource.ReplaceType(pbcatalog.ServiceType, remoteGatewayID)
		service, err := b.dataFetcher.FetchService(context.Background(), serviceID)
		if err != nil {
			b.logger.Trace("error reading exported service", "error", err)
			continue
		} else if service == nil {
			b.logger.Trace("service does not exist, skipping router", "service", serviceID)
			continue
		}

		routers = append(routers, &pbproxystate.Router{
			Match: &pbproxystate.Match{
				ServerNames: []string{
					fmt.Sprintf("*.%s", b.clusterNameForRemoteGateway(remoteGatewayID)),
				},
			},
			Destination: &pbproxystate.Router_L4{
				L4: &pbproxystate.L4Destination{
					Destination: &pbproxystate.L4Destination_Cluster{
						Cluster: &pbproxystate.DestinationCluster{
							Name: b.clusterNameForRemoteGateway(remoteGatewayID),
						},
					},
					StatPrefix: "prefix",
				},
			},
		})
	}

	return routers
}

// wanRouters loops through the ports and consumers for each exported service and generates
// a pbproxystate.Router matching the SNI to the target cluster. Traffic flowing through this
// router originates from a mesh gateway in a remote partition where the mesh gateway mode is
// "local" or from a service in a remote partition where the mesh gateway mode is "remote".
func (b *meshGWProxyStateTemplateBuilder) wanRouters() []*pbproxystate.Router {
	var routers []*pbproxystate.Router

	for _, exportedService := range b.exportedServices {
		serviceID := resource.IDFromReference(exportedService.TargetRef)
		service, err := b.dataFetcher.FetchService(context.Background(), serviceID)
		if err != nil {
			b.logger.Trace("error reading exported service", "error", err)
			continue
		} else if service == nil {
			b.logger.Trace("service does not exist, skipping router", "service", serviceID)
			continue
		}

		for _, port := range service.Data.Ports {
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				continue
			}

			for _, consumer := range exportedService.Consumers {
				routers = append(routers, &pbproxystate.Router{
					Match: &pbproxystate.Match{
						AlpnProtocols: []string{alpnProtocol(port.TargetPort)},
						ServerNames:   []string{b.sniForExportedService(exportedService.TargetRef, consumer)},
					},
					Destination: &pbproxystate.Router_L4{
						L4: &pbproxystate.L4Destination{
							Destination: &pbproxystate.L4Destination_Cluster{
								Cluster: &pbproxystate.DestinationCluster{
									Name: b.clusterNameForExportedService(exportedService.TargetRef, consumer, port.TargetPort),
								},
							},
							StatPrefix: "prefix",
						},
					},
				})
			}
		}
	}

	return routers
}

func (b *meshGWProxyStateTemplateBuilder) clusters() map[string]*pbproxystate.Cluster {
	clusters := map[string]*pbproxystate.Cluster{}

	// Clusters handling incoming traffic from a remote partition
	for _, exportedService := range b.exportedServices {
		serviceID := resource.IDFromReference(exportedService.TargetRef)
		service, err := b.dataFetcher.FetchService(context.Background(), serviceID)
		if err != nil {
			b.logger.Trace("error reading exported service", "error", err)
			continue
		} else if service == nil {
			b.logger.Trace("service does not exist, skipping router", "service", serviceID)
			continue
		}

		for _, port := range service.Data.Ports {
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				continue
			}

			for _, consumer := range exportedService.Consumers {
				clusterName := b.clusterNameForExportedService(exportedService.TargetRef, consumer, port.TargetPort)
				clusters[clusterName] = &pbproxystate.Cluster{
					Name:     clusterName,
					Protocol: pbproxystate.Protocol_PROTOCOL_TCP, // TODO
					Group: &pbproxystate.Cluster_EndpointGroup{
						EndpointGroup: &pbproxystate.EndpointGroup{
							Group: &pbproxystate.EndpointGroup_Dynamic{},
						},
					},
					AltStatName: "prefix",
				}
			}
		}
	}

	// Clusters handling outgoing traffic from the local partition
	for _, remoteGatewayID := range b.remoteGatewayIDs {
		serviceID := resource.ReplaceType(pbcatalog.ServiceType, remoteGatewayID)
		service, err := b.dataFetcher.FetchService(context.Background(), serviceID)
		if err != nil {
			b.logger.Trace("error reading exported service", "error", err)
			continue
		} else if service == nil {
			b.logger.Trace("service does not exist, skipping router", "service", serviceID)
			continue
		}

		clusterName := b.clusterNameForRemoteGateway(remoteGatewayID)
		clusters[clusterName] = &pbproxystate.Cluster{
			Name:     clusterName,
			Protocol: pbproxystate.Protocol_PROTOCOL_TCP, // TODO
			Group: &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: &pbproxystate.EndpointGroup{
					Group: &pbproxystate.EndpointGroup_Dynamic{},
				},
			},
			AltStatName: "prefix",
		}
	}

	// Add null route cluster for any unmatched traffic
	clusters[nullRouteClusterName] = &pbproxystate.Cluster{
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

	return clusters
}

func (b *meshGWProxyStateTemplateBuilder) routes() map[string]*pbproxystate.Route {
	// TODO NET-6428
	return nil
}

func (b *meshGWProxyStateTemplateBuilder) Build() *meshv2beta1.ProxyStateTemplate {
	return &meshv2beta1.ProxyStateTemplate{
		ProxyState: &meshv2beta1.ProxyState{
			Identity:  b.identity(),
			Listeners: b.listeners(),
			Clusters:  b.clusters(),
			Routes:    b.routes(),
		},
		RequiredEndpoints:        b.requiredEndpoints(),
		RequiredLeafCertificates: make(map[string]*pbproxystate.LeafCertificateRef),
		RequiredTrustBundles:     make(map[string]*pbproxystate.TrustBundleRef),
	}
}

// requiredEndpoints loops through the consumers for each exported service
// and adds a pbproxystate.EndpointRef to be hydrated for each cluster.
func (b *meshGWProxyStateTemplateBuilder) requiredEndpoints() map[string]*pbproxystate.EndpointRef {
	requiredEndpoints := make(map[string]*pbproxystate.EndpointRef)

	// Endpoints for clusters handling incoming traffic from another partition
	for _, exportedService := range b.exportedServices {
		serviceID := resource.IDFromReference(exportedService.TargetRef)
		service, err := b.dataFetcher.FetchService(context.Background(), serviceID)
		if err != nil {
			b.logger.Trace("error reading exported service", "error", err)
			continue
		} else if service == nil {
			b.logger.Trace("service does not exist, skipping router", "service", serviceID)
			continue
		}

		for _, port := range service.Data.Ports {
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				continue
			}

			for _, consumer := range exportedService.Consumers {
				clusterName := b.clusterNameForExportedService(exportedService.TargetRef, consumer, port.TargetPort)

				requiredEndpoints[clusterName] = &pbproxystate.EndpointRef{
					Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, serviceID),
					RoutePort: port.TargetPort,
					MeshPort:  "mesh",
				}
			}
		}
	}

	// Endpoints for clusters handling outgoing traffic from the local partition
	for _, remoteGatewayID := range b.remoteGatewayIDs {
		serviceID := resource.ReplaceType(pbcatalog.ServiceType, remoteGatewayID)
		service, err := b.dataFetcher.FetchService(context.Background(), serviceID)
		if err != nil {
			b.logger.Trace("error reading exported service", "error", err)
			continue
		} else if service == nil {
			b.logger.Trace("service does not exist, skipping router", "service", serviceID)
			continue
		}

		clusterName := b.clusterNameForRemoteGateway(remoteGatewayID)

		// In the case of a mesh gateway, the route port and mesh port are the same, since you are always
		// routing to same port that you add in the endpoint. This is different from a sidecar proxy, where
		// the receiving proxy listens on the mesh port and forwards to a different workload port.
		requiredEndpoints[clusterName] = &pbproxystate.EndpointRef{
			Id:        resource.ReplaceType(pbcatalog.ServiceEndpointsType, serviceID),
			MeshPort:  meshgateways.WANPortName,
			RoutePort: meshgateways.WANPortName,
		}
	}

	return requiredEndpoints
}

// clusterNameForExportedService generates a cluster name for a given service
// that is being exported from the local partition to a remote partition. This
// partition may reside in the same datacenter or in a remote datacenter.
func (b *meshGWProxyStateTemplateBuilder) clusterNameForExportedService(serviceRef *pbresource.Reference, consumer *pbmulticluster.ComputedExportedServiceConsumer, port string) string {
	return fmt.Sprintf("%s.%s", port, b.sniForExportedService(serviceRef, consumer))
}

func (b *meshGWProxyStateTemplateBuilder) sniForExportedService(serviceRef *pbresource.Reference, consumer *pbmulticluster.ComputedExportedServiceConsumer) string {
	switch consumer.Tenancy.(type) {
	case *pbmulticluster.ComputedExportedServiceConsumer_Partition:
		return connect.ServiceSNI(serviceRef.Name, "", serviceRef.Tenancy.Namespace, serviceRef.Tenancy.Partition, b.dc, b.trustDomain)
	case *pbmulticluster.ComputedExportedServiceConsumer_Peer:
		return connect.PeeredServiceSNI(serviceRef.Name, serviceRef.Tenancy.Namespace, serviceRef.Tenancy.Partition, b.dc, b.trustDomain)
	default:
		return ""
	}
}

// clusterNameForRemoteGateway generates a cluster name for a given remote mesh
// gateway. This will be used to route traffic from the local partition to the mesh
// gateway for a remote partition.
func (b *meshGWProxyStateTemplateBuilder) clusterNameForRemoteGateway(remoteGatewayID *pbresource.ID) string {
	return connect.GatewaySNI(b.dc, remoteGatewayID.Tenancy.Partition, b.trustDomain)
}

func alpnProtocol(portName string) string {
	return fmt.Sprintf("consul~%s", portName)
}
