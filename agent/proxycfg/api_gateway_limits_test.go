// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"testing"
	"time"

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

func TestAPIGatewayEffectiveUpstreamLimits_PassiveHealthCheckInheritedFromDefaults(t *testing.T) {
	t.Parallel()

	defaults := &structs.UpstreamLimits{
		PassiveHealthCheck: &structs.PassiveHealthCheck{
			Interval:    5 * time.Second,
			MaxFailures: 3,
		},
	}
	// Service overrides only a numeric limit and leaves the health check unset,
	// so the gateway default health check is inherited.
	service := &structs.UpstreamLimits{
		MaxConnections: intPointer(10),
	}

	effective := apiGatewayEffectiveUpstreamLimits(defaults, service)
	require.NotNil(t, effective)
	require.Equal(t, 10, *effective.MaxConnections)
	require.NotNil(t, effective.PassiveHealthCheck)
	require.Equal(t, 5*time.Second, effective.PassiveHealthCheck.Interval)
	require.Equal(t, uint32(3), effective.PassiveHealthCheck.MaxFailures)
}

func TestAPIGatewayEffectiveUpstreamLimits_PassiveHealthCheckServiceOverride(t *testing.T) {
	t.Parallel()

	defaults := &structs.UpstreamLimits{
		PassiveHealthCheck: &structs.PassiveHealthCheck{
			Interval:    5 * time.Second,
			MaxFailures: 3,
		},
	}
	service := &structs.UpstreamLimits{
		PassiveHealthCheck: &structs.PassiveHealthCheck{
			Interval:    10 * time.Second,
			MaxFailures: 7,
		},
	}

	effective := apiGatewayEffectiveUpstreamLimits(defaults, service)
	require.NotNil(t, effective)
	require.NotNil(t, effective.PassiveHealthCheck)
	require.Equal(t, 10*time.Second, effective.PassiveHealthCheck.Interval)
	require.Equal(t, uint32(7), effective.PassiveHealthCheck.MaxFailures)

	// The override must be a clone, not an alias of the service value.
	service.PassiveHealthCheck.MaxFailures = 99
	require.Equal(t, uint32(7), effective.PassiveHealthCheck.MaxFailures)
}
