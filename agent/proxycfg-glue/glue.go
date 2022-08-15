package proxycfgglue

import (
	"context"

	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// Store is the state store interface required for server-local data sources.
type Store interface {
	watch.StateStore

	ExportedServicesForAllPeersByName(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, map[string]structs.ServiceList, error)
	FederationStateList(ws memdb.WatchSet) (uint64, []*structs.FederationState, error)
	GatewayServices(ws memdb.WatchSet, gateway string, entMeta *acl.EnterpriseMeta) (uint64, structs.GatewayServices, error)
	IntentionTopology(ws memdb.WatchSet, target structs.ServiceName, downstreams bool, defaultDecision acl.EnforcementDecision, intentionTarget structs.IntentionTargetType) (uint64, structs.ServiceList, error)
	ServiceDiscoveryChain(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, req discoverychain.CompileRequest) (uint64, *structs.CompiledDiscoveryChain, *configentry.DiscoveryChainSet, error)
	PeeringTrustBundleRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.PeeringTrustBundle, error)
	PeeringTrustBundleList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	TrustBundleListByService(ws memdb.WatchSet, service, dc string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	VirtualIPsForAllImportedServices(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []state.ServiceVirtualIP, error)
}

// CacheCARoots satisfies the proxycfg.CARoots interface by sourcing data from
// the agent cache.
func CacheCARoots(c *cache.Cache) proxycfg.CARoots {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.ConnectCARootName}
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

// CacheServiceGateways satisfies the proxycfg.ServiceGateways interface by
// sourcing data from the agent cache.
func CacheServiceGateways(c *cache.Cache) proxycfg.GatewayServices {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.ServiceGatewaysName}
}

// CacheHTTPChecks satisifies the proxycfg.HTTPChecks interface by sourcing
// data from the agent cache.
func CacheHTTPChecks(c *cache.Cache) proxycfg.HTTPChecks {
	return &cacheProxyDataSource[*cachetype.ServiceHTTPChecksRequest]{c, cachetype.ServiceHTTPChecksName}
}

// CacheIntentionUpstreams satisfies the proxycfg.IntentionUpstreams interface
// by sourcing data from the agent cache.
func CacheIntentionUpstreams(c *cache.Cache) proxycfg.IntentionUpstreams {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.IntentionUpstreamsName}
}

// CacheIntentionUpstreamsDestination satisfies the proxycfg.IntentionUpstreamsDestination interface
// by sourcing data from the agent cache.
func CacheIntentionUpstreamsDestination(c *cache.Cache) proxycfg.IntentionUpstreams {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.IntentionUpstreamsDestinationName}
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
	return c.c.NotifyCallback(ctx, c.t, req, correlationID, dispatchCacheUpdate(ch))
}

func dispatchCacheUpdate(ch chan<- proxycfg.UpdateEvent) cache.Callback {
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
