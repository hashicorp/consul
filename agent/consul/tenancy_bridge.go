// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

// V1TenancyBridge is used by the resource service to access V1 implementations of
// partitions and namespaces. This bridge will be removed when V2 implemenations
// of partitions and namespaces are available.
type V1TenancyBridge struct {
	server *Server
}

func NewV1TenancyBridge(server *Server) *V1TenancyBridge {
	return &V1TenancyBridge{server: server}
}
