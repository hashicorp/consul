// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func (s *ResourceGenerator) appendEntPeeredUpstreamMultiportFilterChains(
	_ *envoy_listener_v3.Listener,
	_ *proxycfg.ConfigSnapshot,
	_ proxycfg.UpstreamID,
	_ string,
	_ string,
	_ filterChainOpts,
) error {
	return nil
}

func (s *ResourceGenerator) appendEntPeeredMultiportFilterChains(
	_ *proxycfg.ConfigSnapshot,
	_ structs.ServiceName,
	_ []string,
	_ string,
	_ *structs.CompiledDiscoveryChain,
	_ bool,
	_ *envoy_core_v3.TransportSocket,
	_ *uint32,
) ([]*envoy_listener_v3.FilterChain, error) {
	return nil, nil
}

func (s *ResourceGenerator) appendEntGatewayOutgoingPeeringServiceMultiportFilterChains(
	_ *envoy_listener_v3.Listener,
	_ string,
	_ *proxycfg.ConfigSnapshot,
) error {
	return nil
}
