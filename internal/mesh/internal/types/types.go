// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

func Register(r resource.Registry) {
	RegisterProxyConfiguration(r)
	RegisterComputedProxyConfiguration(r)
	RegisterDestinations(r)
	RegisterComputedExplicitDestinations(r)
	RegisterComputedImplicitDestinations(r)
	RegisterProxyStateTemplate(r)
	RegisterHTTPRoute(r)
	RegisterTCPRoute(r)
	RegisterGRPCRoute(r)
	RegisterDestinationPolicy(r)
	RegisterComputedRoutes(r)
	RegisterMeshGateway(r)
	RegisterAPIGateway(r)
	RegisterMeshConfiguration(r)
	// todo (v2): uncomment once we implement it.
	// RegisterDestinationsConfiguration(r)
}
