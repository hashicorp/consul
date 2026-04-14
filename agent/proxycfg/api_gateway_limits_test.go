// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestAPIGatewayEffectiveUpstreamLimits_FieldwiseOverride(t *testing.T) {
	t.Parallel()

	defaults := &structs.UpstreamLimits{
		MaxConnections:        intPointer(100),
		MaxPendingRequests:    intPointer(200),
		MaxConcurrentRequests: intPointer(300),
	}
	service := &structs.UpstreamLimits{
		MaxPendingRequests: intPointer(250),
	}

	effective := apiGatewayEffectiveUpstreamLimits(defaults, service)
	require.NotNil(t, effective)
	require.Equal(t, 100, *effective.MaxConnections)
	require.Equal(t, 250, *effective.MaxPendingRequests)
	require.Equal(t, 300, *effective.MaxConcurrentRequests)
}

func TestAPIGatewayEffectiveUpstreamLimits_ServiceOnly(t *testing.T) {
	t.Parallel()

	service := &structs.UpstreamLimits{
		MaxPendingRequests:    intPointer(50),
		MaxConcurrentRequests: intPointer(60),
	}

	effective := apiGatewayEffectiveUpstreamLimits(nil, service)
	require.NotNil(t, effective)
	require.Nil(t, effective.MaxConnections)
	require.Equal(t, 50, *effective.MaxPendingRequests)
	require.Equal(t, 60, *effective.MaxConcurrentRequests)
}

func TestAPIGatewayEffectiveUpstreamLimits_ZeroReturnsNil(t *testing.T) {
	t.Parallel()

	effective := apiGatewayEffectiveUpstreamLimits(nil, &structs.UpstreamLimits{})
	require.Nil(t, effective)
}
