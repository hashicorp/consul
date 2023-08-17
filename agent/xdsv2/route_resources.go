// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

func (pr *ProxyResources) makeRoute(name string) (*envoy_route_v3.RouteConfiguration, error) {
	var route *envoy_route_v3.RouteConfiguration
	// TODO(proxystate): This will make routes in the future. This function should distinguish between static routes
	// inlined into listeners and non-static routes that should be added as top level Envoy resources.
	_, ok := pr.proxyState.Routes[name]
	if !ok {
		// This should not happen with a valid proxy state.
		return nil, fmt.Errorf("could not find route in ProxyState: %s", name)

	}
	return route, nil
}
