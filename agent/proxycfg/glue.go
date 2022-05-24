// TODO(agentless): these glue types belong in the agent package, but moving
// them is a little tricky because the proxycfg tests use them. It should be
// easier to break apart once we no longer depend on cache.Notify directly.
package proxycfg

import (
	"context"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/rpcclient/health"
	"github.com/hashicorp/consul/agent/structs"
)

// HealthWrapper wraps health.Client so that the rest of the proxycfg package
// doesn't need to reference cache.UpdateEvent (it will be extracted into a
// shared library in the future).
type HealthWrapper struct {
	Health *health.Client
}

func (w *HealthWrapper) Notify(
	ctx context.Context,
	req structs.ServiceSpecificRequest,
	correlationID string,
	ch chan<- UpdateEvent,
) error {
	return w.Health.Notify(ctx, req, correlationID, dispatchCacheUpdate(ctx, ch))
}

// CacheWrapper wraps cache.Cache so that the rest of the proxycfg package
// doesn't need to reference cache.UpdateEvent (it will be extracted into a
// shared library in the future).
type CacheWrapper struct {
	Cache *cache.Cache
}

func (w *CacheWrapper) Notify(
	ctx context.Context,
	t string,
	req cache.Request,
	correlationID string,
	ch chan<- UpdateEvent,
) error {
	return w.Cache.NotifyCallback(ctx, t, req, correlationID, dispatchCacheUpdate(ctx, ch))
}

func dispatchCacheUpdate(ctx context.Context, ch chan<- UpdateEvent) cache.Callback {
	return func(ctx context.Context, e cache.UpdateEvent) {
		u := UpdateEvent{
			CorrelationID: e.CorrelationID,
			Result:        e.Result,
			Err:           e.Err,
		}

		select {
		case ch <- u:
		case <-ctx.Done():
		}
	}
}
