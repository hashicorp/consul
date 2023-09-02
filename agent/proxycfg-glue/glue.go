// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"
	"errors"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

// ServerDataSourceDeps contains the dependencies needed for sourcing data from
// server-local sources (e.g. materialized views).
type ServerDataSourceDeps struct {
	Datacenter     string
	ViewStore      *submatview.Store
	EventPublisher *stream.EventPublisher
	Logger         hclog.Logger
	ACLResolver    submatview.ACLResolver
	GetStore       func() Store
}

// Store is the state store interface required for server-local data sources.
type Store interface {
	watch.StateStore

	ExportedServicesForAllPeersByName(ws memdb.WatchSet, dc string, entMeta acl.EnterpriseMeta) (uint64, map[string]structs.ServiceList, error)
	FederationStateList(ws memdb.WatchSet) (uint64, []*structs.FederationState, error)
	GatewayServices(ws memdb.WatchSet, gateway string, entMeta *acl.EnterpriseMeta) (uint64, structs.GatewayServices, error)
	IntentionMatchOne(ws memdb.WatchSet, entry structs.IntentionMatchEntry, matchType structs.IntentionMatchType, destinationType structs.IntentionTargetType) (uint64, structs.SimplifiedIntentions, error)
	IntentionTopology(ws memdb.WatchSet, target structs.ServiceName, downstreams bool, defaultDecision acl.EnforcementDecision, intentionTarget structs.IntentionTargetType) (uint64, structs.ServiceList, error)
	ReadResolvedServiceConfigEntries(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, upstreamIDs []structs.ServiceID, proxyMode structs.ProxyMode) (uint64, *configentry.ResolvedServiceConfigSet, error)
	ServiceDiscoveryChain(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, req discoverychain.CompileRequest) (uint64, *structs.CompiledDiscoveryChain, *configentry.DiscoveryChainSet, error)
	ServiceDump(ws memdb.WatchSet, kind structs.ServiceKind, useKind bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
	PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error)
	PeeringTrustBundleRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.PeeringTrustBundle, error)
	PeeringTrustBundleList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	TrustBundleListByService(ws memdb.WatchSet, service, dc string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	VirtualIPsForAllImportedServices(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []state.ServiceVirtualIP, error)
	CheckConnectServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
	CheckIngressServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta) (uint64, structs.CheckServiceNodes, error)
	CheckServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
}

// CacheCARoots satisfies the proxycfg.CARoots interface by sourcing data from
// the agent cache.
//
// Note: there isn't a server-local equivalent of this data source because
// "agentless" proxies obtain certificates via SDS served by consul-dataplane.
// If SDS is not supported on consul-dataplane, data is sourced from the server agent cache
// even for "agentless" proxies.
func CacheCARoots(c *cache.Cache) proxycfg.CARoots {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.ConnectCARootName}
}

// CacheDatacenters satisfies the proxycfg.Datacenters interface by sourcing
// data from the agent cache.
//
// Note: there isn't a server-local equivalent of this data source because it
// relies on polling (so a more efficient method isn't available).
func CacheDatacenters(c *cache.Cache) proxycfg.Datacenters {
	return &cacheProxyDataSource[*structs.DatacentersRequest]{c, cachetype.CatalogDatacentersName}
}

// CacheServiceGateways satisfies the proxycfg.ServiceGateways interface by
// sourcing data from the agent cache.
func CacheServiceGateways(c *cache.Cache) proxycfg.GatewayServices {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.ServiceGatewaysName}
}

// CachePrepraredQuery satisfies the proxycfg.PreparedQuery interface by
// sourcing data from the agent cache.
//
// Note: there isn't a server-local equivalent of this data source because it
// relies on polling (so a more efficient method isn't available).
func CachePrepraredQuery(c *cache.Cache) proxycfg.PreparedQuery {
	return &cacheProxyDataSource[*structs.PreparedQueryExecuteRequest]{c, cachetype.PreparedQueryName}
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
		select {
		case ch <- newUpdateEvent(e.CorrelationID, e.Result, e.Err):
		case <-ctx.Done():
		}
	}
}

func dispatchBlockingQueryUpdate[ResultType any](ch chan<- proxycfg.UpdateEvent) func(context.Context, string, ResultType, error) {
	return func(ctx context.Context, correlationID string, result ResultType, err error) {
		select {
		case ch <- newUpdateEvent(correlationID, result, err):
		case <-ctx.Done():
		}
	}
}

func newUpdateEvent(correlationID string, result any, err error) proxycfg.UpdateEvent {
	// This roughly matches the logic in agent/submatview.LocalMaterializer.isTerminalError.
	if acl.IsErrNotFound(err) {
		err = proxycfg.TerminalError(err)
	}
	// these are also errors where we should mark them
	// as terminal for the sake of proxycfg, since they require
	// a resubscribe.
	if errors.Is(err, stream.ErrSubForceClosed) || errors.Is(err, stream.ErrShuttingDown) {
		err = proxycfg.TerminalError(err)
	}
	return proxycfg.UpdateEvent{
		CorrelationID: correlationID,
		Result:        result,
		Err:           err,
	}
}
