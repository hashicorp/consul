// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package consul

import "github.com/hashicorp/consul/agent/grpc-external/services/resource"

// V1TenancyBridge is used by the resource service to access V1 implementations of
// partitions and namespaces. This bridge will be removed when V2 implemenations
// of partitions and namespaces are available.
type V1TenancyBridge struct {
	server *Server
}

func NewV1TenancyBridge(server *Server) resource.TenancyBridge {
	return &V1TenancyBridge{server: server}
}
