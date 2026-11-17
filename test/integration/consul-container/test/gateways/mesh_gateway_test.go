// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package gateways

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// registerMeshGateway runs `consul connect envoy -gateway=mesh -register
// -bootstrap`, which registers the gateway with the local agent and prints its
// bootstrap configuration without starting Envoy.
func (e *apiEnv) registerMeshGateway(t *testing.T, name string, extraArgs ...string) libcluster.ExecResult {
	t.Helper()
	args := append([]string{
		"connect", "envoy", "-bootstrap", "-grpc-addr", agentGRPCAddr,
		"-gateway", "mesh", "-register", "-service", name,
	}, extraArgs...)
	return e.cli(t, args...)
}

func meshGatewayEntry(t retry.TestingTB, c *api.Client, name string) *api.ServiceEntry {
	entries, _, err := c.Health().Service(name, "", false, nil)
	require.NoError(t, err)
	if len(entries) == 0 {
		return nil
	}
	require.Len(t, entries, 1, "re-registering a gateway must not create a second instance")
	return entries[0]
}

// TestMeshGateway_RegistrationAndAddressRotation verifies that a mesh gateway
// registers with its LAN and WAN addresses, and that re-running registration
// with new addresses (as happens when a gateway is moved or its load balancer
// changes) updates the existing instance in place rather than adding a new one.
func TestMeshGateway_RegistrationAndAddressRotation(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)
	addrOne, addrTwo := e.bindableAddresses()

	t.Run("registers as a mesh gateway with lan and wan addresses", func(t *testing.T) {
		res := e.registerMeshGateway(t, "mgw",
			"-address", fmt.Sprintf("%s:8443", addrOne),
			"-wan-address", "203.0.113.10:8443",
		)
		require.Equal(t, 0, res.ExitCode, res.Output)

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			entry := meshGatewayEntry(r, e.server, "mgw")
			require.NotNil(r, entry)

			svc := entry.Service
			require.Equal(r, api.ServiceKindMeshGateway, svc.Kind)
			require.Equal(r, addrOne, svc.Address)
			require.Equal(r, 8443, svc.Port)
			require.Equal(r, api.ServiceAddress{Address: addrOne, Port: 8443}, svc.TaggedAddresses["lan"])
			require.Equal(r, api.ServiceAddress{Address: "203.0.113.10", Port: 8443}, svc.TaggedAddresses["wan"])

			var hasListenerCheck bool
			for _, chk := range entry.Checks {
				if chk.ServiceID == "mgw" && chk.Name == "mesh-gateway listening" {
					hasListenerCheck = true
				}
			}
			require.True(r, hasListenerCheck, "the gateway must be health checked on its listener")
		})
	})

	t.Run("re-registering with new addresses updates the instance in place", func(t *testing.T) {
		res := e.registerMeshGateway(t, "mgw",
			"-address", fmt.Sprintf("%s:8444", addrTwo),
			"-wan-address", "203.0.113.20:8444",
		)
		require.Equal(t, 0, res.ExitCode, res.Output)

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			entry := meshGatewayEntry(r, e.server, "mgw")
			require.NotNil(r, entry)

			svc := entry.Service
			require.Equal(r, addrTwo, svc.Address)
			require.Equal(r, 8444, svc.Port)
			require.Equal(r, api.ServiceAddress{Address: addrTwo, Port: 8444}, svc.TaggedAddresses["lan"])
			require.Equal(r, api.ServiceAddress{Address: "203.0.113.20", Port: 8444}, svc.TaggedAddresses["wan"])
		})
	})

	t.Run("dropping the wan address removes the wan tagged address", func(t *testing.T) {
		res := e.registerMeshGateway(t, "mgw", "-address", fmt.Sprintf("%s:8444", addrTwo))
		require.Equal(t, 0, res.ExitCode, res.Output)

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			entry := meshGatewayEntry(r, e.server, "mgw")
			require.NotNil(r, entry)
			wan := entry.Service.TaggedAddresses["wan"]
			require.NotEqual(r, "203.0.113.20", wan.Address, "the previous wan address must not linger")
		})
	})

	t.Run("gateways with different names are independent", func(t *testing.T) {
		res := e.registerMeshGateway(t, "mgw-secondary", "-address", fmt.Sprintf("%s:9443", addrOne))
		require.Equal(t, 0, res.ExitCode, res.Output)

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			secondary := meshGatewayEntry(r, e.server, "mgw-secondary")
			require.NotNil(r, secondary)
			require.Equal(r, 9443, secondary.Service.Port)

			primary := meshGatewayEntry(r, e.server, "mgw")
			require.NotNil(r, primary)
			require.Equal(r, 8444, primary.Service.Port, "registering another gateway must not touch the first")
		})
	})
}

// TestMeshGateway_RegistrationErrors verifies the CLI refuses configurations
// that could never work, instead of registering a broken gateway.
func TestMeshGateway_RegistrationErrors(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)

	t.Run("unknown gateway kind", func(t *testing.T) {
		res := e.cli(t, "connect", "envoy", "-bootstrap", "-grpc-addr", agentGRPCAddr, "-gateway", "bogus", "-register", "-service", "bad-kind")

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "Gateway must be one of: api, terminating, mesh, or ingress")
		require.Nil(t, meshGatewayEntry(t, e.server, "bad-kind"))
	})

	t.Run("a lan address that cannot be bound is rejected", func(t *testing.T) {
		res := e.registerMeshGateway(t, "bad-addr", "-address", "10.255.255.1:8443")

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "will not be bindable")
		require.Nil(t, meshGatewayEntry(t, e.server, "bad-addr"))
	})
}
