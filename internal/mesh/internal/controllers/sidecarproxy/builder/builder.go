// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Builder builds a ProxyStateTemplate.
type Builder struct {
	id                 *pbresource.ID
	proxyStateTemplate *pbmesh.ProxyStateTemplate
	proxyCfg           *pbmesh.ComputedProxyConfiguration
	trustDomain        string
	localDatacenter    string
	defaultAllow       bool
}

func New(
	id *pbresource.ID,
	identity *pbresource.Reference,
	trustDomain string,
	dc string,
	defaultAllow bool,
	proxyCfg *pbmesh.ComputedProxyConfiguration,
) *Builder {
	if !resource.EqualType(pbmesh.ProxyStateTemplateType, id.GetType()) {
		panic(fmt.Sprintf("wrong type: expected pbmesh.ProxyStateTemplate, but got %T", id.Type))
	}

	if !resource.EqualType(pbauth.WorkloadIdentityType, identity.GetType()) {
		panic(fmt.Sprintf("wrong type: expected pbauth.WorkloadIdentityType, but got %T", identity.Type))
	}
	return &Builder{
		id:              id,
		trustDomain:     trustDomain,
		localDatacenter: dc,
		defaultAllow:    defaultAllow,
		proxyCfg:        proxyCfg,
		proxyStateTemplate: &pbmesh.ProxyStateTemplate{
			ProxyState: &pbmesh.ProxyState{
				Identity:  identity,
				Clusters:  make(map[string]*pbproxystate.Cluster),
				Endpoints: make(map[string]*pbproxystate.Endpoints),
				Routes:    make(map[string]*pbproxystate.Route),
			},
			RequiredEndpoints:        make(map[string]*pbproxystate.EndpointRef),
			RequiredLeafCertificates: make(map[string]*pbproxystate.LeafCertificateRef),
			RequiredTrustBundles:     make(map[string]*pbproxystate.TrustBundleRef),
		},
	}
}

func (b *Builder) Build() *pbmesh.ProxyStateTemplate {
	workloadIdentity := b.proxyStateTemplate.ProxyState.Identity.Name
	b.proxyStateTemplate.RequiredLeafCertificates[workloadIdentity] = &pbproxystate.LeafCertificateRef{
		Name:      workloadIdentity,
		Namespace: b.id.Tenancy.Namespace,
		Partition: b.id.Tenancy.Partition,
	}

	b.proxyStateTemplate.RequiredTrustBundles[resource.DefaultPeerName] = &pbproxystate.TrustBundleRef{
		Peer: resource.DefaultPeerName,
	}

	// TODO(peering/v2) add trust bundle refs for all peered upstreams

	finalCleanupOfProxyStateTemplate(b.proxyStateTemplate)

	return b.proxyStateTemplate
}

func finalCleanupOfProxyStateTemplate(pst *pbmesh.ProxyStateTemplate) {
	if pst.ProxyState != nil {
		// Ensure all clusters have names by duplicating them from the map
		// if the above assembly code neglected any.
		for name, cluster := range pst.ProxyState.Clusters {
			if cluster.Name == "" && name != "" {
				cluster.Name = name
			}
		}
	}
}

type ListenerBuilder struct {
	listener *pbproxystate.Listener
	builder  *Builder
}

func (b *Builder) NewListenerBuilder(l *pbproxystate.Listener) *ListenerBuilder {
	return &ListenerBuilder{
		listener: l,
		builder:  b,
	}
}

func (l *ListenerBuilder) buildListener() {
	if l.listener != nil {
		l.builder.proxyStateTemplate.ProxyState.Listeners = append(l.builder.proxyStateTemplate.ProxyState.Listeners, l.listener)
	}
}

type RouterBuilder struct {
	router  *pbproxystate.Router
	builder *ListenerBuilder
}

func (b *ListenerBuilder) NewRouterBuilder(r *pbproxystate.Router) *RouterBuilder {
	return &RouterBuilder{
		router:  r,
		builder: b,
	}
}

func (r *RouterBuilder) buildRouter() {
	r.builder.listener.Routers = append(r.builder.listener.Routers, r.router)
}
