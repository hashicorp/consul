// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	meshv2beta1 "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type proxyStateTemplateBuilder struct {
	workload         *types.DecodedWorkload
	dataFetcher      *fetcher.Fetcher
	dc               string
	exportedServices *types.DecodedComputedExportedServices
	logger           hclog.Logger
	trustDomain      string
}

func NewProxyStateTemplateBuilder(workload *types.DecodedWorkload, exportedServices *types.DecodedComputedExportedServices, logger hclog.Logger, dataFetcher *fetcher.Fetcher, dc, trustDomain string) *proxyStateTemplateBuilder {
	return &proxyStateTemplateBuilder{
		workload:         workload,
		dataFetcher:      dataFetcher,
		dc:               dc,
		exportedServices: exportedServices,
		logger:           logger,
		trustDomain:      trustDomain,
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
	// TODO NET-6429
	return nil
}

func (b *proxyStateTemplateBuilder) clusters() map[string]*pbproxystate.Cluster {
	// TODO NET-6430
	return nil
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
		RequiredEndpoints:        make(map[string]*pbproxystate.EndpointRef),
		RequiredLeafCertificates: make(map[string]*pbproxystate.LeafCertificateRef),
		RequiredTrustBundles:     make(map[string]*pbproxystate.TrustBundleRef),
	}
}
