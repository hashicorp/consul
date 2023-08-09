// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
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
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mapper := bimapper.New(types.ProxyStateTemplateType, catalog.ServiceEndpointsType)
	mgr.Register(xds.Controller(mapper, deps.ProxyUpdater, deps.TrustBundleFetcher))

	destinationsCache := sidecarproxycache.NewDestinationsCache()
	proxyCfgCache := sidecarproxycache.NewProxyConfigurationCache()
	m := sidecarproxymapper.New(destinationsCache, proxyCfgCache)
	mgr.Register(
		sidecarproxy.Controller(destinationsCache, proxyCfgCache, m, deps.TrustDomainFetcher, deps.LocalDatacenter),
	)

	mgr.Register(routes.Controller())
}
