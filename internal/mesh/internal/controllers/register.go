// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/sidecarproxymapper"
)

type Dependencies struct {
	TrustDomainFetcher sidecarproxy.TrustDomainFetcher
	LocalDatacenter    string
}

func Register(mgr *controller.Manager, deps Dependencies) {
	c := sidecarproxycache.NewDestinationsCache()
	m := sidecarproxymapper.New(c)
	mgr.Register(sidecarproxy.Controller(c, m, deps.TrustDomainFetcher, deps.LocalDatacenter))
}
