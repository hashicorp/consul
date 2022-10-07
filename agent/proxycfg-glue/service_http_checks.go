package proxycfgglue

import (
	"context"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// CacheHTTPChecks satisifies the proxycfg.HTTPChecks interface by sourcing
// data from the agent cache.
func CacheHTTPChecks(c *cache.Cache) proxycfg.HTTPChecks {
	return &cacheProxyDataSource[*cachetype.ServiceHTTPChecksRequest]{c, cachetype.ServiceHTTPChecksName}
}

// ServerHTTPChecks satisifies the proxycfg.HTTPChecks interface.
// It sources data from the server agent cache if the service exists in the local state, else
// it is a no-op since the checks can only be performed by local agents.
func ServerHTTPChecks(deps ServerDataSourceDeps, cacheSource proxycfg.HTTPChecks, localState *local.State) proxycfg.HTTPChecks {
	return serverHTTPChecks{deps, cacheSource, localState}
}

type serverHTTPChecks struct {
	deps        ServerDataSourceDeps
	cacheSource proxycfg.HTTPChecks
	localState  *local.State
}

func (c serverHTTPChecks) Notify(ctx context.Context, req *cachetype.ServiceHTTPChecksRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	// if the service exists in the server agent local state, delegate to the cache data source
	if c.localState.ServiceExists(structs.ServiceID{ID: req.ServiceID, EnterpriseMeta: req.EnterpriseMeta}) {
		return c.cacheSource.Notify(ctx, req, correlationID, ch)
	}
	c.deps.Logger.Debug("service-http-checks: no-op")
	return nil
}
