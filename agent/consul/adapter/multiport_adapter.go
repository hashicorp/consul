// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package adapter

import "github.com/hashicorp/consul/agent/structs"

// PopulateLegacyNodeServicePort sets the legacy scalar port from the default
// named port. Older agents do not decode NodeService.Ports from RPC responses,
// so the compatibility value must be populated before the response is sent.
func PopulateLegacyNodeServicePort(service *structs.NodeService) {
	if service != nil && service.Port == 0 && len(service.Ports) > 0 {
		service.Port = service.DefaultPort()
	}
}

// PopulateLegacyServiceNodePorts sets the legacy scalar port on service nodes.
func PopulateLegacyServiceNodePorts(services structs.ServiceNodes) {
	for _, service := range services {
		if service != nil && service.ServicePort == 0 && len(service.ServicePorts) > 0 {
			service.ServicePort = service.ToNodeService().DefaultPort()
		}
	}
}

// PopulateLegacyCheckServiceNodePorts sets the legacy scalar port on the
// services contained in check-service nodes.
func PopulateLegacyCheckServiceNodePorts(nodes structs.CheckServiceNodes) {
	for i := range nodes {
		PopulateLegacyNodeServicePort(nodes[i].Service)
	}
}
