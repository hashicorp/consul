// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
)

// TestMakeAPIGatewayExtAuthzClusters_NoopInCE verifies that no ext_authz mesh
// clusters are emitted for an API Gateway in CE. Emitting mTLS clusters for
// builtin/ext-authz mesh (Service) targets is an enterprise-only feature; the
// enterprise build provides the real implementation.
func TestMakeAPIGatewayExtAuthzClusters_NoopInCE(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	clusters, err := s.makeAPIGatewayExtAuthzClusters(nil, map[proxycfg.UpstreamID]bool{})
	require.NoError(t, err)
	require.Nil(t, clusters)
}

// TestMakeAPIGatewayExtAuthzEndpoints_NoopInCE verifies that no ext_authz mesh
// endpoints are emitted for an API Gateway in CE. See
// TestMakeAPIGatewayExtAuthzClusters_NoopInCE.
func TestMakeAPIGatewayExtAuthzEndpoints_NoopInCE(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	endpoints, err := s.makeAPIGatewayExtAuthzEndpoints(nil, map[proxycfg.UpstreamID]struct{}{})
	require.NoError(t, err)
	require.Nil(t, endpoints)
}
