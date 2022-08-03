package proxycfgglue

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestServerExportedPeeredServices(t *testing.T) {
	nextIndex := indexGenerator()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := state.NewStateStore(nil)

	for _, peer := range []string{"peer-1", "peer-2", "peer-3"} {
		require.NoError(t, store.PeeringWrite(nextIndex(), &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:    testUUID(t),
				Name:  peer,
				State: pbpeering.PeeringState_ACTIVE,
			},
		}))
	}

	require.NoError(t, store.EnsureConfigEntry(nextIndex(), &structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{
			{
				Name: "web",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-1"},
				},
			},
			{
				Name: "db",
				Consumers: []structs.ServiceConsumer{
					{PeerName: "peer-2"},
				},
			},
		},
	}))

	authz := policyAuthorizer(t, `
		service "web" { policy = "read" }
		service "api" { policy = "read" }
		service "db"  { policy = "deny" }
	`)

	eventCh := make(chan proxycfg.UpdateEvent)
	dataSource := ServerExportedPeeredServices(ServerDataSourceDeps{
		GetStore:    func() Store { return store },
		ACLResolver: newStaticResolver(authz),
	})
	require.NoError(t, dataSource.Notify(ctx, &structs.DCSpecificRequest{}, "", eventCh))

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := getEventResult[*structs.IndexedExportedServiceList](t, eventCh)
		require.Equal(t,
			map[string]structs.ServiceList{
				"peer-1": {structs.NewServiceName("web", nil)},
			},
			result.Services,
		)
	})

	testutil.RunStep(t, "update exported services", func(t *testing.T) {
		require.NoError(t, store.EnsureConfigEntry(nextIndex(), &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "web",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "peer-1"},
					},
				},
				{
					Name: "db",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "peer-2"},
					},
				},
				{
					Name: "api",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "peer-1"},
						{PeerName: "peer-3"},
					},
				},
			},
		}))

		result := getEventResult[*structs.IndexedExportedServiceList](t, eventCh)
		require.Equal(t,
			map[string]structs.ServiceList{
				"peer-1": {
					structs.NewServiceName("api", nil),
					structs.NewServiceName("web", nil),
				},
				"peer-3": {
					structs.NewServiceName("api", nil),
				},
			},
			result.Services,
		)
	})
}
