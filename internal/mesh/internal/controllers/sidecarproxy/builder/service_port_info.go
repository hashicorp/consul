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
		for _, address := range ep.Addresses {

			hasAddressLevelPorts := false
			if len(address.Ports) > 0 {
				hasAddressLevelPorts = true
			}

			// if address has specific ports, add those to the seen array
			for _, portName := range address.Ports {
				// check that it is not service mesh port because we don't
				// want to add that to the service ports map.
				epPort, epOK := ep.Ports[portName]
				if isMeshPort(epPort) {
					continue
				}

				_, ok := seen[portName]
				if ok {
					// port is in the seen map
					if epOK {
						// port is also in the endpoints list which we want to verify.

						// if this port has been seen already, add the current endpoint as a seenBy if it is not already
						// present.
						if seen[portName].seenBy == nil {
							// port is in the seen map but has no seenBy indexes.
							seen[portName].seenBy = append([]*int{}, &epIdx)
						} else if len(seen[portName].seenBy)-1 < epIdx {
							// port is in the seen map and has seenBy indexes but not this one.
							seen[portName].seenBy = append(seen[portName].seenBy, &epIdx)
						}
						// else do nothing since this endpoint has already marked it as seen.
					}
				} else {
					// port is not yet in the seen map
					seenBy := append([]*int{}, &epIdx)
					seen[portName] = &seenData{port: epPort, seenBy: seenBy}
				}
			}

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

				// if address specifies a subset, it has already been accounted
				// for in the seen list.
				if hasAddressLevelPorts {
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
