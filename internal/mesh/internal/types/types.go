// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

const (
	GroupName      = "mesh"
	VersionV2beta1 = "v2beta1"
	CurrentVersion = VersionV2beta1
)

func Register(r resource.Registry) {
	RegisterProxyConfiguration(r)
	RegisterUpstreams(r)
	RegisterUpstreamsConfiguration(r)
	RegisterProxyStateTemplate(r)
	RegisterHTTPRoute(r)
	RegisterTCPRoute(r)
	RegisterGRPCRoute(r)
	RegisterDestinationPolicy(r)
	RegisterComputedRoutes(r)
}
