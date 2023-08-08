// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/mapper"
)

type Dependencies struct {
	TrustDomainFetcher sidecarproxy.TrustDomainFetcher
}

func Register(mgr *controller.Manager, deps Dependencies) {
	c := cache.New()
	m := mapper.New(c)
	mgr.Register(sidecarproxy.Controller(c, m, deps.TrustDomainFetcher))
}
