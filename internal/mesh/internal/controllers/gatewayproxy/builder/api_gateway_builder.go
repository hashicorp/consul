// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
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
	services    []*pbcatalog.Service
	tcpRoutes   []*meshv2beta1.TCPRoute
	apiGateway  *meshv2beta1.APIGateway
	logger      hclog.Logger
	trustDomain string
}

func NewAPIGWProxyStateTemplateBuilder(workload *types.DecodedWorkload, services []*pbcatalog.Service, tcpRoutes []*meshv2beta1.TCPRoute, apiGateway *meshv2beta1.APIGateway, logger hclog.Logger, dataFetcher *fetcher.Fetcher, dc, trustDomain string) *apiGWProxyStateTemplateBuilder {
	return &apiGWProxyStateTemplateBuilder{
		workload:    workload,
		dataFetcher: dataFetcher,
		services:    services,
		tcpRoutes:   tcpRoutes,
		apiGateway:  apiGateway,
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

func (b *apiGWProxyStateTemplateBuilder) listeners() []*pbproxystate.Listener {
	// TODO NET-7985
	var listeners []*pbproxystate.Listener

	address := b.workload.Data.Addresses[0]
	for idx, portName := range address.Ports {

		workloadPort, ok := b.workload.Data.Ports[portName]
		if !ok {
			b.logger.Trace("port does not exist for workload", "port name", portName)
			continue
		}
		listeners = append(listeners, b.listener(fmt.Sprintf("default-%d", idx), address, workloadPort.Port, pbproxystate.Direction_DIRECTION_INBOUND, b.routers()))
	}

	b.logger.Trace("listeners for apigw pst", "listeners", listeners)
	return listeners
}

func (b *apiGWProxyStateTemplateBuilder) listener(name string, address *pbcatalog.WorkloadAddress, port uint32, _ pbproxystate.Direction, routers []*pbproxystate.Router) *pbproxystate.Listener {
	// TODO NET-7985
	return &pbproxystate.Listener{
		Name:      name,
		Direction: pbproxystate.Direction_DIRECTION_INBOUND,
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

func (b *apiGWProxyStateTemplateBuilder) clusters() map[string]*pbproxystate.Cluster {
	clusters := map[string]*pbproxystate.Cluster{}

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

	// TODO NET-7984
	return clusters
}

func (b *apiGWProxyStateTemplateBuilder) routers() []*pbproxystate.Router {
	return []*pbproxystate.Router{}
}

func (b *apiGWProxyStateTemplateBuilder) routes() map[string]*pbproxystate.Route {
	// TODO NET-7986
	return nil
}

func (b *apiGWProxyStateTemplateBuilder) Build() *meshv2beta1.ProxyStateTemplate {
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

func (b *apiGWProxyStateTemplateBuilder) requiredEndpoints() map[string]*pbproxystate.EndpointRef {
	requiredEndpoints := make(map[string]*pbproxystate.EndpointRef)

	return requiredEndpoints
}
