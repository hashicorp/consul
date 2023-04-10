// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package discoverychain

import "github.com/hashicorp/consul/agent/structs"

func synthesizeTCPRouteDiscoveryChain(route structs.TCPRouteConfigEntry) []structs.IngressService {
	services := make([]structs.IngressService, 0, len(route.Services))
	for _, service := range route.Services {
		ingress := structs.IngressService{
			Name:           service.Name,
			EnterpriseMeta: service.EnterpriseMeta,
		}

		services = append(services, ingress)
	}

	return services
}
