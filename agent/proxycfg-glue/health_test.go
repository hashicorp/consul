package proxycfgglue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerHealth(t *testing.T) {
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

		dataSource := ServerHealth(ServerDataSourceDeps{Datacenter: "dc1"}, remoteSource)
		err := dataSource.Notify(ctx, req, correlationID, ch)
		require.Equal(t, result, err)
	})

	t.Run("local queries are served from a materialized view", func(t *testing.T) {
		// Note: the view is tested more thoroughly in the agent/rpcclient/health
		// package, so this is more of a high-level integration test with the local
		// materializer.
		const (
			index       uint64 = 123
			datacenter         = "dc1"
			serviceName        = "web"
		)

		logger := testutil.Logger(t)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		store := submatview.NewStore(logger)
		go store.Run(ctx)

		publisher := stream.NewEventPublisher(10 * time.Second)
		publisher.RegisterHandler(pbsubscribe.Topic_ServiceHealth,
			func(stream.SubscribeRequest, stream.SnapshotAppender) (uint64, error) { return index, nil },
			true)
		go publisher.Run(ctx)

		dataSource := ServerHealth(ServerDataSourceDeps{
			Datacenter:     datacenter,
			ACLResolver:    newStaticResolver(acl.ManageAll()),
			ViewStore:      store,
			EventPublisher: publisher,
			Logger:         logger,
		}, nil)

		eventCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(ctx, &structs.ServiceSpecificRequest{
			Datacenter:  datacenter,
			ServiceName: serviceName,
		}, "", eventCh))

		testutil.RunStep(t, "initial state", func(t *testing.T) {
			result := getEventResult[*structs.IndexedCheckServiceNodes](t, eventCh)
			require.Empty(t, result.Nodes)
		})

		testutil.RunStep(t, "register services", func(t *testing.T) {
			publisher.Publish([]stream.Event{
				{
					Index: index + 1,
					Topic: pbsubscribe.Topic_ServiceHealth,
					Payload: &state.EventPayloadCheckServiceNode{
						Op: pbsubscribe.CatalogOp_Register,
						Value: &structs.CheckServiceNode{
							Node:    &structs.Node{Node: "node1"},
							Service: &structs.NodeService{Service: serviceName},
						},
					},
				},
				{
					Index: index + 1,
					Topic: pbsubscribe.Topic_ServiceHealth,
					Payload: &state.EventPayloadCheckServiceNode{
						Op: pbsubscribe.CatalogOp_Register,
						Value: &structs.CheckServiceNode{
							Node:    &structs.Node{Node: "node2"},
							Service: &structs.NodeService{Service: serviceName},
						},
					},
				},
			})

			result := getEventResult[*structs.IndexedCheckServiceNodes](t, eventCh)
			require.Len(t, result.Nodes, 2)
		})

		testutil.RunStep(t, "deregister service", func(t *testing.T) {
			publisher.Publish([]stream.Event{
				{
					Index: index + 2,
					Topic: pbsubscribe.Topic_ServiceHealth,
					Payload: &state.EventPayloadCheckServiceNode{
						Op: pbsubscribe.CatalogOp_Deregister,
						Value: &structs.CheckServiceNode{
							Node:    &structs.Node{Node: "node2"},
							Service: &structs.NodeService{Service: serviceName},
						},
					},
				},
			})

			result := getEventResult[*structs.IndexedCheckServiceNodes](t, eventCh)
			require.Len(t, result.Nodes, 1)
		})
	})
}

func newMockHealth(t *testing.T) *mockHealth {
	mock := &mockHealth{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

type mockHealth struct {
	mock.Mock
}

func (m *mockHealth) Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return m.Called(ctx, req, correlationID, ch).Error(0)
}
