// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	meshv2beta1 "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type proxyStateTemplateBuilder struct {
	exportedServices *types.DecodedComputedExportedServices
	workload         *types.DecodedWorkload
}

func NewProxyStateTemplateBuilder(workload *types.DecodedWorkload, exportedServices *types.DecodedComputedExportedServices) *proxyStateTemplateBuilder {
	return &proxyStateTemplateBuilder{
		exportedServices: exportedServices,
		workload:         workload,
	}
}

func (b *proxyStateTemplateBuilder) identity() *pbresource.Reference {
	return &pbresource.Reference{
		Name:    b.workload.Data.Identity,
		Tenancy: b.workload.Id.Tenancy,
		Type:    pbauth.WorkloadIdentityType,
	}
}

func (b *proxyStateTemplateBuilder) listeners() []*pbproxystate.Listener {
	var address string
	if len(b.workload.Data.Addresses) > 0 {
		address = b.workload.Data.Addresses[0].Host
	}

	listener := &pbproxystate.Listener{
		Name:      xdscommon.PublicListenerName,
		Direction: pbproxystate.Direction_DIRECTION_INBOUND,
		BindAddress: &pbproxystate.Listener_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: address,
				Port: b.workload.Data.Ports["wan"].Port,
			},
		},
		Capabilities: []pbproxystate.Capability{
			pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION,
		},
		DefaultRouter: &pbproxystate.Router{
			Match:       &pbproxystate.Match{ServerNames: []string{""}},
			Destination: &pbproxystate.Router_Sni{},
			InboundTls:  &pbproxystate.TransportSocket{},
		},
		Routers: b.routers(),
	}

	// TODO NET-6429
	return []*pbproxystate.Listener{listener}
}

// routers loops through the consumers for each exported service and generates a
// pbproxystate.Router matching the SNI to the cluster name, which matches the SNI
func (b *proxyStateTemplateBuilder) routers() []*pbproxystate.Router {
	var routers []*pbproxystate.Router

	for _, service := range b.exportedServices.Data.Consumers {
		for _, consumer := range service.Consumers {
			sni := clusterName(service.TargetRef, consumer)
			routers = append(routers, &pbproxystate.Router{
				Match:       &pbproxystate.Match{ServerNames: []string{sni}},
				Destination: &pbproxystate.Router_Sni{},
			})
		}
	}

	return routers
}

// clusters loops through the consumers for each exported service
// and generates a pbproxystate.Cluster per service-consumer pairing.
func (b *proxyStateTemplateBuilder) clusters() map[string]*pbproxystate.Cluster {
	clusters := map[string]*pbproxystate.Cluster{}

	for _, service := range b.exportedServices.Data.Consumers {
		for _, consumer := range service.Consumers {
			clusterName := clusterName(service.TargetRef, consumer)
			clusters[clusterName] = &pbproxystate.Cluster{
				Name:     clusterName,
				Protocol: pbproxystate.Protocol_PROTOCOL_UNSPECIFIED, // TODO
			}
		}
	}

	clusters["my-fancy-cluster"] = &pbproxystate.Cluster{
		Name: "my-fancy-cluster",
		Group: &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: &pbproxystate.EndpointGroup{
				Group: &pbproxystate.EndpointGroup_Dynamic{},
			},
		},
		AltStatName: "prefix",
		Protocol:    pbproxystate.Protocol_PROTOCOL_TCP, // TODO
	}

	return clusters
}

// requiredEndpoints loops through the consumers for each exported service
// and adds a pbproxystate.EndpointRef to be hydrated for each cluster.
func (b *proxyStateTemplateBuilder) requiredEndpoints() map[string]*pbproxystate.EndpointRef {
	requiredEndpoints := map[string]*pbproxystate.EndpointRef{}

	for _, service := range b.exportedServices.Data.Consumers {
		for _, consumer := range service.Consumers {
			clusterName := clusterName(service.TargetRef, consumer)
			requiredEndpoints[clusterName] = &pbproxystate.EndpointRef{
				Id: &pbresource.ID{
					Name:    service.TargetRef.Name,
					Type:    pbcatalog.ServiceEndpointsType,
					Tenancy: service.TargetRef.Tenancy,
				},
				Port: service.TargetRef.Section,
			}
		}
	}

	requiredEndpoints["my-fancy-cluster"] = &pbproxystate.EndpointRef{
		Id: &pbresource.ID{
			Name:    "backend",
			Type:    pbcatalog.ServiceEndpointsType,
			Tenancy: &pbresource.Tenancy{Namespace: "default", Partition: "default"},
		},
		Port: "8080",
	}

	return requiredEndpoints
}

func (b *proxyStateTemplateBuilder) endpoints() map[string]*pbproxystate.Endpoints {
	// TODO NET-6431
	return nil
}

func (b *proxyStateTemplateBuilder) routes() map[string]*pbproxystate.Route {
	// TODO NET-6428
	return nil
}

func (b *proxyStateTemplateBuilder) Build() *meshv2beta1.ProxyStateTemplate {
	return &meshv2beta1.ProxyStateTemplate{
		ProxyState: &meshv2beta1.ProxyState{
			Identity:  b.identity(),
			Listeners: b.listeners(),
			Clusters:  b.clusters(),
			Endpoints: b.endpoints(),
			Routes:    b.routes(),
		},
		RequiredEndpoints:        b.requiredEndpoints(),
		RequiredLeafCertificates: make(map[string]*pbproxystate.LeafCertificateRef),
		RequiredTrustBundles:     make(map[string]*pbproxystate.TrustBundleRef),
	}
}

func clusterName(serviceRef *pbresource.Reference, consumer *pbmulticluster.ComputedExportedServicesConsumer) string {
	switch tConsumer := consumer.ConsumerTenancy.(type) {
	case *pbmulticluster.ComputedExportedServicesConsumer_Partition:
		return fmt.Sprintf("%s-%d", serviceRef.Name, tConsumer.Partition)
	case *pbmulticluster.ComputedExportedServicesConsumer_Peer:
		return fmt.Sprintf("%s-%d", serviceRef.Name, tConsumer.Peer)
	default:
		return ""
	}
}
