// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

import (
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
)

// makeAPIGatewayExtAuthzClusters is a no-op in CE. Emitting mTLS clusters for
// builtin/ext-authz mesh (Service) targets on an API Gateway is an
// enterprise-only feature; the enterprise build provides the real implementation.
func (s *ResourceGenerator) makeAPIGatewayExtAuthzClusters(_ *proxycfg.ConfigSnapshot, _ map[proxycfg.UpstreamID]bool) ([]proto.Message, error) {
	return nil, nil
}

// makeAPIGatewayExtAuthzEndpoints is a no-op in CE. See makeAPIGatewayExtAuthzClusters.
func (s *ResourceGenerator) makeAPIGatewayExtAuthzEndpoints(_ *proxycfg.ConfigSnapshot, _ map[proxycfg.UpstreamID]struct{}) ([]proto.Message, error) {
	return nil, nil
}
