// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/mapper"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/mapper"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/xds"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
)

type Dependencies struct {
	TrustDomainFetcher sidecarproxy.TrustDomainFetcher
	TrustBundleFetcher xds.TrustBundleFetcher
	ProxyUpdater       xds.ProxyUpdater
}

func Register(mgr *controller.Manager, deps Dependencies) {
	c := cache.New()
	m := mapper.New(c)
	mapper := bimapper.New(types.ProxyStateTemplateType, catalog.ServiceEndpointsType)
	mgr.Register(xds.Controller(mapper, deps.ProxyUpdater, deps.TrustBundleFetcher))
	mgr.Register(sidecarproxy.Controller(c, m, deps.TrustDomainFetcher))
}
