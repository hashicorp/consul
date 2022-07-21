package proxycfgglue

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerPeeredUpstreams(t *testing.T) {
	const (
		index    uint64 = 123
		nodeName        = "node-1"
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := state.NewStateStore(nil)
	enableVirtualIPs(t, store)

	registerService := func(t *testing.T, index uint64, peerName, serviceName string) {
		require.NoError(t, store.EnsureRegistration(index, &structs.RegisterRequest{
			Node:           nodeName,
			Service:        &structs.NodeService{Service: serviceName, ID: serviceName},
			PeerName:       peerName,
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}))

		require.NoError(t, store.EnsureRegistration(index, &structs.RegisterRequest{
			Node: nodeName,
			Service: &structs.NodeService{
				Service: fmt.Sprintf("%s-proxy", serviceName),
				Kind:    structs.ServiceKindConnectProxy,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: serviceName,
				},
			},
			PeerName:       peerName,
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		}))
	}

	registerService(t, index, "peer-1", "web")

	eventCh := make(chan proxycfg.UpdateEvent)
	dataSource := ServerPeeredUpstreams(ServerDataSourceDeps{
		GetStore: func() Store { return store },
	})
	require.NoError(t, dataSource.Notify(ctx, &structs.PartitionSpecificRequest{EnterpriseMeta: *acl.DefaultEnterpriseMeta()}, "", eventCh))

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)
		require.Len(t, result.Services, 1)
		require.Equal(t, "peer-1", result.Services[0].Peer)
		require.Equal(t, "web", result.Services[0].ServiceName.Name)
	})

	testutil.RunStep(t, "register another service", func(t *testing.T) {
		registerService(t, index+1, "peer-2", "db")

		result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)
		require.Len(t, result.Services, 2)
	})

	testutil.RunStep(t, "deregister service", func(t *testing.T) {
		require.NoError(t, store.DeleteService(index+2, nodeName, "web", acl.DefaultEnterpriseMeta(), "peer-1"))

		result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)
		require.Len(t, result.Services, 1)
	})
}

func enableVirtualIPs(t *testing.T, store *state.Store) {
	t.Helper()

	require.NoError(t, store.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataVirtualIPsEnabled,
		Value: "true",
	}))
}
