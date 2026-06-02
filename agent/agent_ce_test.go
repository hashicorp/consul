// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestAgent_consulConfig_Reporting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	hcl := `
		reporting {
			license {
				enabled = true
			}
		}
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	require.Equal(t, false, a.consulConfig().Reporting.License.Enabled)
}

func TestAgent_consulConfig_Reporting_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	hcl := `
		reporting {
		}
	`
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	require.Equal(t, false, a.consulConfig().Reporting.License.Enabled)
}

func TestValidateEnterpriseMeshPortConfig(t *testing.T) {
	t.Parallel()

	t.Run("plain multiport service stays valid", func(t *testing.T) {
		err := validateEnterpriseMeshPortConfig(&structs.NodeService{
			Service: "api",
			Ports:   structs.ServicePorts{{Name: "http", Port: 8080, Default: true}},
		})
		require.NoError(t, err)
	})

	t.Run("multiport sidecar registration is rejected", func(t *testing.T) {
		err := validateEnterpriseMeshPortConfig(&structs.NodeService{
			Service: "api",
			Ports:   structs.ServicePorts{{Name: "http", Port: 8080, Default: true}},
			Connect: structs.ServiceConnect{SidecarService: &structs.ServiceDefinition{}},
		})
		require.ErrorContains(t, err, "named service ports in the service mesh require Consul Enterprise")
	})

	t.Run("destination port upstreams are rejected", func(t *testing.T) {
		err := validateEnterpriseMeshPortConfig(&structs.NodeService{
			Kind: structs.ServiceKindConnectProxy,
			Proxy: structs.ConnectProxyConfig{
				Upstreams: structs.Upstreams{{
					DestinationName: "db",
					DestinationPort: "admin",
					LocalBindPort:   7000,
				}},
			},
		})
		require.ErrorContains(t, err, "destination port routing requires Consul Enterprise")
	})
}
