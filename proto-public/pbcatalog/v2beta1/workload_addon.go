// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

import "golang.org/x/exp/slices"

func (w *Workload) GetMeshPortName() (string, bool) {
	var meshPort string

	for portName, port := range w.GetPorts() {
		if port.Protocol == Protocol_PROTOCOL_MESH {
			meshPort = portName
			return meshPort, true
		}
	}

	return "", false
}

func (w *Workload) IsMeshEnabled() bool {
	_, ok := w.GetMeshPortName()
	return ok
}

func (w *Workload) GetNonExternalAddressesForPort(portName string) []*WorkloadAddress {
	var addresses []*WorkloadAddress

	// If this port doesn't exist on a workload, return nil.
	if _, ok := w.GetPorts()[portName]; !ok {
		return nil
	}

	for _, address := range w.GetAddresses() {
		if address.GetExternal() {
			// Skip external addresses.
			continue
		}

		// If there are no ports, that means this port is selected.
		// Otherwise, check if the port is explicitly selected by this address
		if len(address.Ports) == 0 || slices.Contains(address.GetPorts(), portName) {
			addresses = append(addresses, address)
		}
	}

	return addresses
}

func (w *Workload) GetFirstNonExternalMeshAddress() *WorkloadAddress {
	// Find mesh port.
	meshPort, ok := w.GetMeshPortName()
	if !ok {
		return nil
	}

	// Check if the workload has a specific address for the mesh port.
	meshAddresses := w.GetNonExternalAddressesForPort(meshPort)

	// If there are no mesh addresses, return. This should be impossible.
	if len(meshAddresses) == 0 {
		return nil
	}

	// If there are more than one mesh address, use the first one in the list.
	return meshAddresses[0]
}

func (w *Workload) GetPortsByProtocol() map[Protocol][]string {
	if w == nil {
		return nil
	}

	out := make(map[Protocol][]string, len(w.GetPorts()))
	for name, port := range w.GetPorts() {
		out[port.GetProtocol()] = append(out[port.GetProtocol()], name)
	}

	return out
}
