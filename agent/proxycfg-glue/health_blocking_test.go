package proxycfgglue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestServerHealthBlocking(t *testing.T) {
	t.Run("remote queries are delegated to the remote source", func(t *testing.T) {
		var (
			ctx           = context.Background()
			req           = &structs.ServiceSpecificRequest{Datacenter: "dc2"}
			correlationID = "correlation-id"
			ch            = make(chan<- proxycfg.UpdateEvent)
			result        = errors.New("KABOOM")
		)

		remoteSource := newMockHealth(t)
		remoteSource.On("Notify", ctx, req, correlationID, ch).Return(result)

		dataSource := ServerHealthBlocking(ServerDataSourceDeps{Datacenter: "dc1"}, remoteSource)
		err := dataSource.Notify(ctx, req, correlationID, ch)
		require.Equal(t, result, err)
	})

	t.Run("services notify correctly", func(t *testing.T) {
		const (
			datacenter  = "dc1"
			serviceName = "web"
		)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		store := state.NewStateStore(nil)
		aclResolver := newStaticResolver(acl.ManageAll())
		dataSource := ServerHealthBlocking(ServerDataSourceDeps{
			GetStore:    func() Store { return store },
			Datacenter:  datacenter,
			ACLResolver: aclResolver,
			Logger:      testutil.Logger(t),
		}, nil)
		dataSource.watchTimeout = 1 * time.Second

		// Watch for all events
		eventCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(ctx, &structs.ServiceSpecificRequest{
			Datacenter:  datacenter,
			ServiceName: serviceName,
		}, "", eventCh))

		// Watch for a subset of events
		filteredCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(ctx, &structs.ServiceSpecificRequest{
			Datacenter:  datacenter,
			ServiceName: serviceName,
			QueryOptions: structs.QueryOptions{
				Filter: "Service.ID == \"web1\"",
			},
		}, "", filteredCh))

		testutil.RunStep(t, "initial state", func(t *testing.T) {
			result := getEventResult[*structs.IndexedCheckServiceNodes](t, eventCh)
			require.Empty(t, result.Nodes)
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, filteredCh)
			require.Empty(t, result.Nodes)
		})

		testutil.RunStep(t, "register services", func(t *testing.T) {
			require.NoError(t, store.EnsureRegistration(10, &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
				Service: &structs.NodeService{
					ID:      serviceName + "1",
					Service: serviceName,
					Port:    80,
				},
			}))
			result := getEventResult[*structs.IndexedCheckServiceNodes](t, eventCh)
			require.Len(t, result.Nodes, 1)
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, filteredCh)
			require.Len(t, result.Nodes, 1)

			require.NoError(t, store.EnsureRegistration(11, &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
				Service: &structs.NodeService{
					ID:      serviceName + "2",
					Service: serviceName,
					Port:    81,
				},
			}))
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, eventCh)
			require.Len(t, result.Nodes, 2)
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, filteredCh)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "web1", result.Nodes[0].Service.ID)
		})

		testutil.RunStep(t, "deregister service", func(t *testing.T) {
			require.NoError(t, store.DeleteService(12, "foo", serviceName+"1", nil, ""))
			result := getEventResult[*structs.IndexedCheckServiceNodes](t, eventCh)
			require.Len(t, result.Nodes, 1)
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, filteredCh)
			require.Len(t, result.Nodes, 0)
		})

		testutil.RunStep(t, "acl enforcement", func(t *testing.T) {
			require.NoError(t, store.EnsureRegistration(11, &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
				Service: &structs.NodeService{
					Service: serviceName + "-sidecar-proxy",
					Kind:    structs.ServiceKindConnectProxy,
					Proxy: structs.ConnectProxyConfig{
						DestinationServiceName: serviceName,
					},
				},
			}))

			authzDeny := policyAuthorizer(t, ``)
			authzAllow := policyAuthorizer(t, `
				node_prefix "" { policy = "read" }
				service_prefix "web" { policy = "read" }
			`)

			// Start a stream where insufficient permissions are denied
			aclDenyCh := make(chan proxycfg.UpdateEvent)
			aclResolver.SwapAuthorizer(authzDeny)
			require.NoError(t, dataSource.Notify(ctx, &structs.ServiceSpecificRequest{
				Connect:     true,
				Datacenter:  datacenter,
				ServiceName: serviceName,
			}, "", aclDenyCh))
			require.ErrorContains(t, getEventError(t, aclDenyCh), "Permission denied")

			// Adding ACL permissions will send valid data
			aclResolver.SwapAuthorizer(authzAllow)
			time.Sleep(dataSource.watchTimeout)
			result := getEventResult[*structs.IndexedCheckServiceNodes](t, aclDenyCh)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "web-sidecar-proxy", result.Nodes[0].Service.Service)

			// Start a stream where sufficient permissions are allowed
			aclAllowCh := make(chan proxycfg.UpdateEvent)
			aclResolver.SwapAuthorizer(authzAllow)
			require.NoError(t, dataSource.Notify(ctx, &structs.ServiceSpecificRequest{
				Connect:     true,
				Datacenter:  datacenter,
				ServiceName: serviceName,
			}, "", aclAllowCh))
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, aclAllowCh)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "web-sidecar-proxy", result.Nodes[0].Service.Service)

			// Removing ACL permissions will send empty data
			aclResolver.SwapAuthorizer(authzDeny)
			time.Sleep(dataSource.watchTimeout)
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, aclAllowCh)
			require.Len(t, result.Nodes, 0)

			// Adding ACL permissions will send valid data
			aclResolver.SwapAuthorizer(authzAllow)
			time.Sleep(dataSource.watchTimeout)
			result = getEventResult[*structs.IndexedCheckServiceNodes](t, aclAllowCh)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "web-sidecar-proxy", result.Nodes[0].Service.Service)
		})
	})
}
