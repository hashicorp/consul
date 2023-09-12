// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
)

// servicePortInfo is a struct used by the destination builder so that it can pre-process
// what is the service mesh port and what are the distinct ports across the endpoints.
// This pre-processing reduces the iterations during the BuildDestinations processing
// would need to get the ports, check if one was the mesh port, know whether it already
// has a cluster created for it and so on.

type servicePortInfo struct {
	// meshPortName is the name of the port with Mesh Protocol.
	meshPortName string
	// meshPort is the port with Mesh Protocol.
	meshPort *pbcatalog.WorkloadPort
	// servicePorts are the distinct ports that need clusters and routers for given destination.
	// servicePorts do not include the mesh port and are called servicePorts because they
	// belong to the service.
	servicePorts map[string]*pbcatalog.WorkloadPort
}

// newServicePortInfo builds a servicePointInfo given a serviceEndpoints struct. It pre-process
// what the service mesh port and the distinct service ports across the endpoints for a destination.
// The following occurs during pre-processing:
//   - a port must be exposed to at least one workload address on every workload in
//     the service to be a service port.  Otherwise, the system would risk errors.
//   - a Workload can optionally define ports specific to workload address.  If no
//     ports are specified for a workload address, then all the destination ports are
//     used.
func newServicePortInfo(serviceEndpoints *pbcatalog.ServiceEndpoints) *servicePortInfo {
	spInfo := &servicePortInfo{
		servicePorts: make(map[string]*pbcatalog.WorkloadPort),
	}
	type seenData struct {
		port   *pbcatalog.WorkloadPort
		seenBy []*int
	}
	seen := make(map[string]*seenData)
	for epIdx, ep := range serviceEndpoints.GetEndpoints() {
		for range ep.Addresses {
			// iterate through endpoint ports and set the mesh port
			// as well as all endpoint ports for this workload if there
			// are no specific workload ports.
			for epPortName, epPort := range ep.Ports {
				// look to set mesh port
				if isMeshPort(epPort) {
					spInfo.meshPortName = epPortName
					spInfo.meshPort = epPort
					continue
				}

				// otherwise, add all ports for this endpoint.
				portData, ok := seen[epPortName]
				if ok {
					portData.seenBy = append(portData.seenBy, &epIdx)
				} else {
					seenBy := append([]*int{}, &epIdx)
					seen[epPortName] = &seenData{port: epPort, seenBy: seenBy}
				}
			}
		}
	}

	for portName, portData := range seen {
		// make sure each port is seen by all endpoints
		if len(portData.seenBy) == len(serviceEndpoints.GetEndpoints()) {
			spInfo.servicePorts[portName] = portData.port
		}
	}
	return spInfo
}

func isMeshPort(port *pbcatalog.WorkloadPort) bool {
	return port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH
}
