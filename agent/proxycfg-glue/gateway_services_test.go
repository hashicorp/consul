// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerGatewayServices(t *testing.T) {
	const index uint64 = 123

	t.Run("ingress gateway", func(t *testing.T) {
		store := state.NewStateStore(nil)

		authz := policyAuthorizer(t, `
			service "igw" { policy = "read" }
			service "web" { policy = "read" }
			service "db" { policy = "read" }
		`)

		require.NoError(t, store.EnsureConfigEntry(index, &structs.IngressGatewayConfigEntry{
			Name: "igw",
			Listeners: []structs.IngressListener{
				{
					Protocol: "tcp",
					Services: []structs.IngressService{
						{Name: "web"},
					},
				},
				{
					Protocol: "tcp",
					Services: []structs.IngressService{
						{Name: "db"},
					},
				},
				{
					Protocol: "tcp",
					Services: []structs.IngressService{
						{Name: "no-access"},
					},
				},
			},
		}))

		dataSource := ServerGatewayServices(ServerDataSourceDeps{
			ACLResolver: newStaticResolver(authz),
			GetStore:    func() Store { return store },
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(context.Background(), &structs.ServiceSpecificRequest{ServiceName: "igw"}, "", eventCh))

		testutil.RunStep(t, "initial state", func(t *testing.T) {
			result := getEventResult[*structs.IndexedGatewayServices](t, eventCh)
			require.Len(t, result.Services, 2)
		})

		testutil.RunStep(t, "remove service mapping", func(t *testing.T) {
			require.NoError(t, store.EnsureConfigEntry(index+1, &structs.IngressGatewayConfigEntry{
				Name: "igw",
				Listeners: []structs.IngressListener{
					{
						Protocol: "tcp",
						Services: []structs.IngressService{
							{Name: "web"},
						},
					},
				},
			}))

			result := getEventResult[*structs.IndexedGatewayServices](t, eventCh)
			require.Len(t, result.Services, 1)
		})
	})

	t.Run("terminating gateway", func(t *testing.T) {
		store := state.NewStateStore(nil)

		authz := policyAuthorizer(t, `
			service "tgw" { policy = "read" }
			service "web" { policy = "read" }
			service "db" { policy = "read" }
		`)

		require.NoError(t, store.EnsureConfigEntry(index, &structs.TerminatingGatewayConfigEntry{
			Name: "tgw",
			Services: []structs.LinkedService{
				{Name: "web"},
				{Name: "db"},
				{Name: "no-access"},
			},
		}))

		dataSource := ServerGatewayServices(ServerDataSourceDeps{
			ACLResolver: newStaticResolver(authz),
			GetStore:    func() Store { return store },
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(context.Background(), &structs.ServiceSpecificRequest{ServiceName: "tgw"}, "", eventCh))

		testutil.RunStep(t, "initial state", func(t *testing.T) {
			result := getEventResult[*structs.IndexedGatewayServices](t, eventCh)
			require.Len(t, result.Services, 2)
		})

		testutil.RunStep(t, "remove service mapping", func(t *testing.T) {
			require.NoError(t, store.EnsureConfigEntry(index+1, &structs.TerminatingGatewayConfigEntry{
				Name: "tgw",
				Services: []structs.LinkedService{
					{Name: "web"},
				},
			}))

			result := getEventResult[*structs.IndexedGatewayServices](t, eventCh)
			require.Len(t, result.Services, 1)
		})
	})

	t.Run("no access to gateway", func(t *testing.T) {
		store := state.NewStateStore(nil)

		authz := policyAuthorizer(t, `
			service "tgw" { policy = "deny" }
			service "web" { policy = "read" }
			service "db" { policy = "read" }
		`)

		require.NoError(t, store.EnsureConfigEntry(index, &structs.TerminatingGatewayConfigEntry{
			Name: "tgw",
			Services: []structs.LinkedService{
				{Name: "web"},
				{Name: "db"},
			},
		}))

		dataSource := ServerGatewayServices(ServerDataSourceDeps{
			ACLResolver: newStaticResolver(authz),
			GetStore:    func() Store { return store },
		})

		eventCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(context.Background(), &structs.ServiceSpecificRequest{ServiceName: "tgw"}, "", eventCh))

		err := getEventError(t, eventCh)
		require.True(t, acl.IsErrPermissionDenied(err), "expected permission denied error")
	})
}
