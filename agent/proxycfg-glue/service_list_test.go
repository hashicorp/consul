// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerServiceList(t *testing.T) {
	t.Run("remote queries are delegated to the remote source", func(t *testing.T) {
		var (
			ctx           = context.Background()
			req           = &structs.DCSpecificRequest{Datacenter: "dc2"}
			correlationID = "correlation-id"
			ch            = make(chan<- proxycfg.UpdateEvent)
			result        = errors.New("KABOOM")
		)

		remoteSource := newMockServiceList(t)
		remoteSource.On("Notify", ctx, req, correlationID, ch).Return(result)

		dataSource := ServerServiceList(ServerDataSourceDeps{Datacenter: "dc1"}, remoteSource)
		err := dataSource.Notify(ctx, req, correlationID, ch)
		require.Equal(t, result, err)
	})

	t.Run("local queries are served from a materialized view", func(t *testing.T) {
		const (
			index      uint64 = 123
			datacenter        = "dc1"
		)

		logger := testutil.Logger(t)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		store := submatview.NewStore(logger)
		go store.Run(ctx)

		publisher := stream.NewEventPublisher(10 * time.Second)
		publisher.RegisterHandler(pbsubscribe.Topic_ServiceList,
			func(stream.SubscribeRequest, stream.SnapshotAppender) (uint64, error) { return index, nil },
			true)
		go publisher.Run(ctx)

		dataSource := ServerServiceList(ServerDataSourceDeps{
			Datacenter:     datacenter,
			ACLResolver:    newStaticResolver(acl.ManageAll()),
			ViewStore:      store,
			EventPublisher: publisher,
			Logger:         logger,
		}, nil)

		eventCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(ctx, &structs.DCSpecificRequest{Datacenter: datacenter}, "", eventCh))

		testutil.RunStep(t, "initial state", func(t *testing.T) {
			result := getEventResult[*structs.IndexedServiceList](t, eventCh)
			require.Empty(t, result.Services)
		})

		testutil.RunStep(t, "register services", func(t *testing.T) {
			publisher.Publish([]stream.Event{
				{
					Index: index + 1,
					Topic: pbsubscribe.Topic_ServiceList,
					Payload: &state.EventPayloadServiceListUpdate{
						Op:   pbsubscribe.CatalogOp_Register,
						Name: "web",
					},
				},
				{
					Index: index + 1,
					Topic: pbsubscribe.Topic_ServiceList,
					Payload: &state.EventPayloadServiceListUpdate{
						Op:   pbsubscribe.CatalogOp_Register,
						Name: "db",
					},
				},
			})

			result := getEventResult[*structs.IndexedServiceList](t, eventCh)
			require.Len(t, result.Services, 2)

			var names []string
			for _, service := range result.Services {
				names = append(names, service.Name)
			}
			require.ElementsMatch(t, names, []string{"web", "db"})
		})

		testutil.RunStep(t, "deregister service", func(t *testing.T) {
			publisher.Publish([]stream.Event{
				{
					Index: index + 2,
					Topic: pbsubscribe.Topic_ServiceList,
					Payload: &state.EventPayloadServiceListUpdate{
						Op:   pbsubscribe.CatalogOp_Deregister,
						Name: "web",
					},
				},
			})

			result := getEventResult[*structs.IndexedServiceList](t, eventCh)
			require.Len(t, result.Services, 1)
			require.Equal(t, "db", result.Services[0].Name)
		})
	})
}

func newMockServiceList(t *testing.T) *mockServiceList {
	mock := &mockServiceList{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

type mockServiceList struct {
	mock.Mock
}

func (m *mockServiceList) Notify(ctx context.Context, req *structs.DCSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return m.Called(ctx, req, correlationID, ch).Error(0)
}
