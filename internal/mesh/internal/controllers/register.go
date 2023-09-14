// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"context"

	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/sidecarproxymapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
)

type Dependencies struct {
	TrustDomainFetcher sidecarproxy.TrustDomainFetcher
	LocalDatacenter    string
	TrustBundleFetcher xds.TrustBundleFetcher
	ProxyUpdater       xds.ProxyUpdater
	LeafCertManager    *leafcert.Manager
}

func Register(mgr *controller.Manager, deps Dependencies) {
	endpointsMapper := bimapper.New(types.ProxyStateTemplateType, catalog.ServiceEndpointsType)
	leafMapper := &xds.LeafMapper{
		Mapper: bimapper.New(types.ProxyStateTemplateType, xds.InternalLeafType),
	}
	leafCancels := &xds.LeafCancels{
		Cancels: make(map[string]context.CancelFunc),
	}
	mgr.Register(xds.Controller(endpointsMapper, deps.ProxyUpdater, deps.TrustBundleFetcher, deps.LeafCertManager, leafMapper, leafCancels, deps.LocalDatacenter))

	var (
		destinationsCache   = sidecarproxycache.NewDestinationsCache()
		proxyCfgCache       = sidecarproxycache.NewProxyConfigurationCache()
		computedRoutesCache = sidecarproxycache.NewComputedRoutesCache()
		m                   = sidecarproxymapper.New(destinationsCache, proxyCfgCache, computedRoutesCache)
	)
	mgr.Register(
		sidecarproxy.Controller(destinationsCache, proxyCfgCache, computedRoutesCache, m, deps.TrustDomainFetcher, deps.LocalDatacenter),
	)

	mgr.Register(routes.Controller())
}
