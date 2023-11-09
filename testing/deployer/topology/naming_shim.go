// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

// Deprecated: SortedWorkloads
func (n *Node) SortedServices() []*Service {
	return n.SortedWorkloads()
}

// Deprecated: mapifyWorkloads
func mapifyServices(services []*Service) map[ServiceID]*Service {
	return mapifyWorkloads(services)
}
