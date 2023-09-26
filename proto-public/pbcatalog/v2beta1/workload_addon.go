package catalogv2beta1

import "golang.org/x/exp/slices"

func (w *Workload) GetMeshPortName() (string, bool) {
	var meshPort string

	for portName, port := range w.Ports {
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

	for _, address := range w.Addresses {
		if address.External {
			// Skip external addresses.
			continue
		}

		// If there are no ports, that means this port is selected.
		// Otherwise, check if the port is explicitly selected by this address
		if len(address.Ports) == 0 || slices.Contains(address.Ports, portName) {
			addresses = append(addresses, address)
		}
	}

	return addresses
}

func (w *Workload) GetPortsByProtocol() map[Protocol][]string {
	if w == nil {
		return nil
	}

	out := make(map[Protocol][]string, len(w.Ports))
	for name, port := range w.Ports {
		out[port.GetProtocol()] = append(out[port.GetProtocol()], name)
	}

	return out
}
