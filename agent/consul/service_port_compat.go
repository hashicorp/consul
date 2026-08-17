// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import "github.com/hashicorp/consul/agent/structs"

// populateLegacyNodeServicePort sets the legacy scalar port from the default
// named port. Older agents do not decode NodeService.Ports from RPC responses,
// so the compatibility value must be populated before the response is sent.
func populateLegacyNodeServicePort(service *structs.NodeService) {
	if service != nil && service.Port == 0 && len(service.Ports) > 0 {
		service.Port = service.DefaultPort()
	}
}

func populateLegacyServiceNodePorts(services structs.ServiceNodes) {
	for _, service := range services {
		if service != nil && service.ServicePort == 0 && len(service.ServicePorts) > 0 {
			service.ServicePort = service.ToNodeService().DefaultPort()
		}
	}
}

func populateLegacyCheckServiceNodePorts(nodes structs.CheckServiceNodes) {
	for i := range nodes {
		populateLegacyNodeServicePort(nodes[i].Service)
	}
}
