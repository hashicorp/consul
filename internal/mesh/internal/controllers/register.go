// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/mapper"
)

type Dependencies struct {
	TrustDomainFetcher sidecar_proxy.TrustDomainFetcher
}

func Register(mgr *controller.Manager, deps Dependencies) {
	c := cache.New()
	m := mapper.New(c)
	mgr.Register(sidecar_proxy.Controller(c, m, deps.TrustDomainFetcher))
}
