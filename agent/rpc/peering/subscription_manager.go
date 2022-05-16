package peering

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto/pbservice"
)

type MaterializedViewStore interface {
	Get(ctx context.Context, req submatview.Request) (submatview.Result, error)
	Notify(ctx context.Context, req submatview.Request, cID string, ch chan<- cache.UpdateEvent) error
}

type SubscriptionBackend interface {
	Subscriber
	Store() Store
}

// subscriptionManager handlers requests to subscribe to events from an events publisher.
type subscriptionManager struct {
	logger    hclog.Logger
	viewStore MaterializedViewStore
	backend   SubscriptionBackend

	// watchedServices is a map of exported services to a cancel function for their subscription notifier.
	watchedServices map[structs.ServiceName]context.CancelFunc
}

// TODO(peering): Maybe centralize so that there is a single manager per datacenter, rather than per peering.
func newSubscriptionManager(ctx context.Context, logger hclog.Logger, backend SubscriptionBackend) *subscriptionManager {
	logger = logger.Named("subscriptions")
	store := submatview.NewStore(logger.Named("viewstore"))
	go store.Run(ctx)

	return &subscriptionManager{
		logger:          logger,
		viewStore:       store,
		backend:         backend,
		watchedServices: make(map[structs.ServiceName]context.CancelFunc),
	}
}

// subscribe returns a channel that will contain updates to exported service instances for a given peer.
func (m *subscriptionManager) subscribe(ctx context.Context, peerID string) <-chan cache.UpdateEvent {
	updateCh := make(chan cache.UpdateEvent, 1)
	go m.syncSubscriptions(ctx, peerID, updateCh)

	return updateCh
}

func (m *subscriptionManager) syncSubscriptions(ctx context.Context, peerID string, updateCh chan<- cache.UpdateEvent) {
	waiter := &retry.Waiter{
		MinFailures: 1,
		Factor:      500 * time.Millisecond,
		MaxWait:     60 * time.Second,
		Jitter:      retry.NewJitter(100),
	}

	for {
		if err := m.syncSubscriptionsAndBlock(ctx, peerID, updateCh); err != nil {
			m.logger.Error("failed to sync subscriptions", "error", err)
		}

		if err := waiter.Wait(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			m.logger.Error("failed to wait before re-trying sync", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// syncSubscriptionsAndBlock ensures that the subscriptions to the subscription backend
// match the list of services exported to the peer.
func (m *subscriptionManager) syncSubscriptionsAndBlock(ctx context.Context, peerID string, updateCh chan<- cache.UpdateEvent) error {
	store := m.backend.Store()

	ws := memdb.NewWatchSet()
	ws.Add(store.AbandonCh())
	ws.Add(ctx.Done())

	// Get exported services for peer id
	_, services, err := store.ExportedServicesForPeer(ws, peerID)
	if err != nil {
		return fmt.Errorf("failed to watch exported services for peer %q: %w", peerID, err)
	}

	// seen contains the set of exported service names and is used to reconcile the list of watched services.
	seen := make(map[structs.ServiceName]struct{})

	// Ensure there is a subscription for each service exported to the peer.
	for _, svc := range services {
		seen[svc] = struct{}{}

		if _, ok := m.watchedServices[svc]; ok {
			// Exported service is already being watched, nothing to do.
			continue
		}

		notifyCtx, cancel := context.WithCancel(ctx)
		m.watchedServices[svc] = cancel

		if err := m.Notify(notifyCtx, svc, updateCh); err != nil {
			m.logger.Error("failed to subscribe to service", "service", svc.String())
			continue
		}
	}

	// For every subscription without an exported service, call the associated cancel fn.
	for svc, cancel := range m.watchedServices {
		if _, ok := seen[svc]; !ok {
			cancel()

			// Send an empty event to the stream handler to trigger sending a DELETE message.
			// Cancelling the subscription context above is necessary, but does not yield a useful signal on its own.
			updateCh <- cache.UpdateEvent{
				CorrelationID: subExportedService + svc.String(),
				Result:        &pbservice.IndexedCheckServiceNodes{},
			}
		}
	}

	// Block for any changes to the state store.
	ws.WatchCtx(ctx)
	return nil
}

const (
	subExportedService = "exported-service:"
)

// Notify the given channel when there are updates to the requested service.
func (m *subscriptionManager) Notify(ctx context.Context, svc structs.ServiceName, updateCh chan<- cache.UpdateEvent) error {
	sr := newExportedServiceRequest(m.logger, svc, m.backend)
	return m.viewStore.Notify(ctx, sr, subExportedService+svc.String(), updateCh)
}
