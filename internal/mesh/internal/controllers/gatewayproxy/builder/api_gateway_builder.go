// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/proxytarget"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
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
	clusters := map[string]*pbproxystate.Cluster{}
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

func (b *apiGWProxyStateTemplateBuilder) listenersAndRoutes() ([]*pbproxystate.Listener, map[string]*pbproxystate.Route) {
	listeners := []*pbproxystate.Listener{}
	routes := map[string]*pbproxystate.Route{}

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
