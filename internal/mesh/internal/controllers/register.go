// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"context"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/meshconfiguration"

	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/explicitdestinations"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/explicitdestinations/mapper"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/meshgateways"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/proxyconfiguration"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/workloadselectionmapper"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

type Dependencies struct {
	TrustDomainFetcher sidecarproxy.TrustDomainFetcher
	LocalDatacenter    string
	DefaultAllow       bool
	TrustBundleFetcher xds.TrustBundleFetcher
	ProxyUpdater       xds.ProxyUpdater
	LeafCertManager    *leafcert.Manager
}

func Register(mgr *controller.Manager, deps Dependencies) {
	endpointsMapper := bimapper.New(pbmesh.ProxyStateTemplateType, pbcatalog.ServiceEndpointsType)
	leafMapper := &xds.LeafMapper{
		Mapper: bimapper.New(pbmesh.ProxyStateTemplateType, xds.InternalLeafType),
	}
	leafCancels := &xds.LeafCancels{
		Cancels: make(map[string]context.CancelFunc),
	}
	mgr.Register(xds.Controller(endpointsMapper, deps.ProxyUpdater, deps.TrustBundleFetcher, deps.LeafCertManager, leafMapper, leafCancels, deps.LocalDatacenter))

	mgr.Register(
		sidecarproxy.Controller(cache.New(), deps.TrustDomainFetcher, deps.LocalDatacenter, deps.DefaultAllow),
	)

	mgr.Register(gatewayproxy.Controller(cache.New()))

	mgr.Register(routes.Controller())

	mgr.Register(proxyconfiguration.Controller(workloadselectionmapper.New[*pbmesh.ProxyConfiguration](pbmesh.ComputedProxyConfigurationType)))
	mgr.Register(explicitdestinations.Controller(mapper.New()))

	mgr.Register(meshgateways.Controller())
	mgr.Register(meshconfiguration.Controller())
}
