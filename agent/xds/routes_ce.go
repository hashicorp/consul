// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

import (
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func (s *ResourceGenerator) appendEntMeshGatewayPeeredMultiportRoutes(
	resources []proto.Message,
	_ *proxycfg.ConfigSnapshot,
	_ structs.ServiceName,
	_ *structs.CompiledDiscoveryChain,
	_ *envoy_route_v3.RouteConfiguration,
) ([]proto.Message, error) {
	return resources, nil
}
