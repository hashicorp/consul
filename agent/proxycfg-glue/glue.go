package proxycfgglue

import (
	"context"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/rpcclient/health"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// CacheCARoots satisfies the proxycfg.CARoots interface by sourcing data from
// the agent cache.
func CacheCARoots(c *cache.Cache) proxycfg.CARoots {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.ConnectCARootName}
}

// CacheCompiledDiscoveryChain satisfies the proxycfg.CompiledDiscoveryChain
// interface by sourcing data from the agent cache.
func CacheCompiledDiscoveryChain(c *cache.Cache) proxycfg.CompiledDiscoveryChain {
	return &cacheProxyDataSource[*structs.DiscoveryChainRequest]{c, cachetype.CompiledDiscoveryChainName}
}

// CacheConfigEntry satisfies the proxycfg.ConfigEntry interface by sourcing
// data from the agent cache.
func CacheConfigEntry(c *cache.Cache) proxycfg.ConfigEntry {
	return &cacheProxyDataSource[*structs.ConfigEntryQuery]{c, cachetype.ConfigEntryName}
}

// CacheConfigEntryList satisfies the proxycfg.ConfigEntryList interface by
// sourcing data from the agent cache.
func CacheConfigEntryList(c *cache.Cache) proxycfg.ConfigEntryList {
	return &cacheProxyDataSource[*structs.ConfigEntryQuery]{c, cachetype.ConfigEntryListName}
}

// CacheDatacenters satisfies the proxycfg.Datacenters interface by sourcing
// data from the agent cache.
func CacheDatacenters(c *cache.Cache) proxycfg.Datacenters {
	return &cacheProxyDataSource[*structs.DatacentersRequest]{c, cachetype.CatalogDatacentersName}
}

// CacheFederationStateListMeshGateways satisfies the proxycfg.FederationStateListMeshGateways
// interface by sourcing data from the agent cache.
func CacheFederationStateListMeshGateways(c *cache.Cache) proxycfg.FederationStateListMeshGateways {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.FederationStateListMeshGatewaysName}
}

// CacheGatewayServices satisfies the proxycfg.GatewayServices interface by
// sourcing data from the agent cache.
func CacheGatewayServices(c *cache.Cache) proxycfg.GatewayServices {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.GatewayServicesName}
}

// CacheHTTPChecks satisifies the proxycfg.HTTPChecks interface by sourcing
// data from the agent cache.
func CacheHTTPChecks(c *cache.Cache) proxycfg.HTTPChecks {
	return &cacheProxyDataSource[*cachetype.ServiceHTTPChecksRequest]{c, cachetype.ServiceHTTPChecksName}
}

// CacheIntentions satisfies the proxycfg.Intentions interface by sourcing data
// from the agent cache.
func CacheIntentions(c *cache.Cache) proxycfg.Intentions {
	return &cacheProxyDataSource[*structs.IntentionQueryRequest]{c, cachetype.IntentionMatchName}
}

// CacheIntentionUpstreams satisfies the proxycfg.IntentionUpstreams interface
// by sourcing data from the agent cache.
func CacheIntentionUpstreams(c *cache.Cache) proxycfg.IntentionUpstreams {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.IntentionUpstreamsName}
}

// CacheInternalServiceDump satisfies the proxycfg.InternalServiceDump
// interface by sourcing data from the agent cache.
func CacheInternalServiceDump(c *cache.Cache) proxycfg.InternalServiceDump {
	return &cacheProxyDataSource[*structs.ServiceDumpRequest]{c, cachetype.InternalServiceDumpName}
}

// CacheLeafCertificate satisifies the proxycfg.LeafCertificate interface by
// sourcing data from the agent cache.
func CacheLeafCertificate(c *cache.Cache) proxycfg.LeafCertificate {
	return &cacheProxyDataSource[*cachetype.ConnectCALeafRequest]{c, cachetype.ConnectCALeafName}
}

// CachePrepraredQuery satisfies the proxycfg.PreparedQuery interface by
// sourcing data from the agent cache.
func CachePrepraredQuery(c *cache.Cache) proxycfg.PreparedQuery {
	return &cacheProxyDataSource[*structs.PreparedQueryExecuteRequest]{c, cachetype.PreparedQueryName}
}

// CacheResolvedServiceConfig satisfies the proxycfg.ResolvedServiceConfig
// interface by sourcing data from the agent cache.
func CacheResolvedServiceConfig(c *cache.Cache) proxycfg.ResolvedServiceConfig {
	return &cacheProxyDataSource[*structs.ServiceConfigRequest]{c, cachetype.ResolvedServiceConfigName}
}

// CacheServiceList satisfies the proxycfg.ServiceList interface by sourcing
// data from the agent cache.
func CacheServiceList(c *cache.Cache) proxycfg.ServiceList {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.CatalogServiceListName}
}

// CacheTrustBundle satisfies the proxycfg.TrustBundle interface by sourcing
// data from the agent cache.
func CacheTrustBundle(c *cache.Cache) proxycfg.TrustBundle {
	return &cacheProxyDataSource[*pbpeering.TrustBundleReadRequest]{c, cachetype.TrustBundleReadName}
}

// CacheTrustBundleList satisfies the proxycfg.TrustBundleList interface by sourcing
// data from the agent cache.
func CacheTrustBundleList(c *cache.Cache) proxycfg.TrustBundleList {
	return &cacheProxyDataSource[*pbpeering.TrustBundleListByServiceRequest]{c, cachetype.TrustBundleListName}
}

// CacheExportedPeeredServices satisfies the proxycfg.ExportedPeeredServices
// interface by sourcing data from the agent cache.
func CacheExportedPeeredServices(c *cache.Cache) proxycfg.ExportedPeeredServices {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.ExportedPeeredServicesName}
}

// cacheProxyDataSource implements a generic wrapper around the agent cache to
// provide data to the proxycfg.Manager.
type cacheProxyDataSource[ReqType cache.Request] struct {
	c *cache.Cache
	t string
}

// Notify satisfies the interfaces used by proxycfg.Manager to source data by
// subscribing to notifications from the agent cache.
func (c *cacheProxyDataSource[ReqType]) Notify(
	ctx context.Context,
	req ReqType,
	correlationID string,
	ch chan<- proxycfg.UpdateEvent,
) error {
	return c.c.NotifyCallback(ctx, c.t, req, correlationID, dispatchCacheUpdate(ctx, ch))
}

// Health wraps health.Client so that the proxycfg package doesn't need to
// reference cache.UpdateEvent directly.
func Health(client *health.Client) proxycfg.Health {
	return &healthWrapper{client}
}

type healthWrapper struct {
	client *health.Client
}

func (h *healthWrapper) Notify(
	ctx context.Context,
	req *structs.ServiceSpecificRequest,
	correlationID string,
	ch chan<- proxycfg.UpdateEvent,
) error {
	return h.client.Notify(ctx, *req, correlationID, dispatchCacheUpdate(ctx, ch))
}

func dispatchCacheUpdate(ctx context.Context, ch chan<- proxycfg.UpdateEvent) cache.Callback {
	return func(ctx context.Context, e cache.UpdateEvent) {
		u := proxycfg.UpdateEvent{
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
