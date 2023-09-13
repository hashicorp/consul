// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"context"

	"github.com/hashicorp/consul/agent/leafcert"
	catalogapi "github.com/hashicorp/consul/api/catalog/v2beta1"
	meshapi "github.com/hashicorp/consul/api/mesh/v2beta1"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/sidecarproxymapper"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
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
	endpointsMapper := bimapper.New(meshapi.ProxyStateTemplateType, catalogapi.ServiceEndpointsType)
	leafMapper := &xds.LeafMapper{
		Mapper: bimapper.New(meshapi.ProxyStateTemplateType, xds.InternalLeafType),
	}
	leafCancels := &xds.LeafCancels{
		Cancels: make(map[string]context.CancelFunc),
	}
	mgr.Register(xds.Controller(endpointsMapper, deps.ProxyUpdater, deps.TrustBundleFetcher, deps.LeafCertManager, leafMapper, leafCancels, deps.LocalDatacenter))

	var (
		destinationsCache   = sidecarproxycache.NewDestinationsCache()
		proxyCfgCache       = sidecarproxycache.NewProxyConfigurationCache()
		computedRoutesCache = sidecarproxycache.NewComputedRoutesCache()
		identitiesCache     = sidecarproxycache.NewIdentitiesCache()
		m                   = sidecarproxymapper.New(destinationsCache, proxyCfgCache, computedRoutesCache, identitiesCache)
	)
	mgr.Register(
		sidecarproxy.Controller(destinationsCache, proxyCfgCache, computedRoutesCache, identitiesCache, m, deps.TrustDomainFetcher, deps.LocalDatacenter, deps.DefaultAllow),
	)

	mgr.Register(routes.Controller())
}
