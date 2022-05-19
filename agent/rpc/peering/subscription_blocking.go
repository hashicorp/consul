package peering

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto/pbservice"
)

// This file contains direct state store functions that need additional
// management to have them emit events. Ideally these would go through
// streaming machinery instead to be cheaper.

func (m *subscriptionManager) notifyExportedServicesForPeerID(ctx context.Context, state *subscriptionState, peerID string) {
	// syncSubscriptionsAndBlock ensures that the subscriptions to the subscription backend
	// match the list of services exported to the peer.
	m.syncViaBlockingQuery(ctx, "exported-services", func(ctx context.Context, store Store, ws memdb.WatchSet) (interface{}, error) {
		// Get exported services for peer id
		_, list, err := store.ExportedServicesForPeer(ws, peerID)
		if err != nil {
			return nil, fmt.Errorf("failed to watch exported services for peer %q: %w", peerID, err)
		}

		return list, nil
	}, subExportedServiceList, state.updateCh)
}

// TODO: add a new streaming subscription type to list-by-kind-and-partition since we're getting evictions
func (m *subscriptionManager) notifyMeshGatewaysForPartition(ctx context.Context, state *subscriptionState, partition string) {
	m.syncViaBlockingQuery(ctx, "mesh-gateways", func(ctx context.Context, store Store, ws memdb.WatchSet) (interface{}, error) {
		// Fetch our current list of all mesh gateways.
		entMeta := structs.DefaultEnterpriseMetaInPartition(partition)
		idx, nodes, err := store.ServiceDump(ws, structs.ServiceKindMeshGateway, true, entMeta, structs.DefaultPeerKeyword)
		if err != nil {
			return nil, fmt.Errorf("failed to watch mesh gateways services for partition %q: %w", partition, err)
		}
		if idx == 0 {
			idx = 1
		}

		// convert back to a protobuf flavor
		result := &pbservice.IndexedCheckServiceNodes{
			Index: idx,
			Nodes: make([]*pbservice.CheckServiceNode, len(nodes)),
		}
		for i, csn := range nodes {
			result.Nodes[i] = pbservice.NewCheckServiceNodeFromStructs(&csn)
		}

		return result, nil
	}, subMeshGateway+partition, state.updateCh)
}

func (m *subscriptionManager) syncViaBlockingQuery(
	ctx context.Context,
	queryType string,
	queryFn func(ctx context.Context, store Store, ws memdb.WatchSet) (interface{}, error),
	correlationID string,
	updateCh chan<- cache.UpdateEvent,
) {
	waiter := &retry.Waiter{
		MinFailures: 1,
		Factor:      500 * time.Millisecond,
		MaxWait:     60 * time.Second,
		Jitter:      retry.NewJitter(100),
	}

	logger := m.logger
	if queryType != "" {
		logger = m.logger.With("queryType", queryType)
	}

	store := m.backend.Store()

	for {
		ws := memdb.NewWatchSet()
		ws.Add(store.AbandonCh())
		ws.Add(ctx.Done())

		if result, err := queryFn(ctx, store, ws); err != nil {
			logger.Error("failed to sync from query", "error", err)
		} else {
			// Block for any changes to the state store.
			updateCh <- cache.UpdateEvent{
				CorrelationID: correlationID,
				Result:        result,
			}
			ws.WatchCtx(ctx)
		}

		if err := waiter.Wait(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.Error("failed to wait before re-trying sync", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
