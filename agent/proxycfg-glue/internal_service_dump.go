// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfgglue

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
)

// CacheInternalServiceDump satisfies the proxycfg.InternalServiceDump
// interface by sourcing data from the agent cache.
func CacheInternalServiceDump(c *cache.Cache) proxycfg.InternalServiceDump {
	return &cacheInternalServiceDump{c}
}

// cacheInternalServiceDump wraps the underlying cache-type to return a simpler
// subset of the response (as this is all we use in proxycfg).
type cacheInternalServiceDump struct {
	c *cache.Cache
}

func (c *cacheInternalServiceDump) Notify(ctx context.Context, req *structs.ServiceDumpRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	dispatch := dispatchCacheUpdate(ch)

	return c.c.NotifyCallback(ctx, cachetype.InternalServiceDumpName, req, correlationID,
		func(ctx context.Context, event cache.UpdateEvent) {
			if r, _ := event.Result.(*structs.IndexedNodesWithGateways); r != nil {
				event.Result = &structs.IndexedCheckServiceNodes{
					Nodes:     r.Nodes,
					QueryMeta: r.QueryMeta,
				}
			}
			dispatch(ctx, event)
		})
}

// ServerInternalServiceDump satisfies the proxycfg.InternalServiceDump
// interface by sourcing data from a blocking query against the server's
// state store.
func ServerInternalServiceDump(deps ServerDataSourceDeps, remoteSource proxycfg.InternalServiceDump) proxycfg.InternalServiceDump {
	return &serverInternalServiceDump{deps, remoteSource}
}

type serverInternalServiceDump struct {
	deps         ServerDataSourceDeps
	remoteSource proxycfg.InternalServiceDump
}

func (s *serverInternalServiceDump) Notify(ctx context.Context, req *structs.ServiceDumpRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	if req.Datacenter != s.deps.Datacenter {
		return s.remoteSource.Notify(ctx, req, correlationID, ch)
	}

	filter, err := bexpr.CreateFilter(req.Filter, nil, structs.CheckServiceNodes{})
	if err != nil {
		return err
	}

	// This is just the small subset of the Internal.ServiceDump RPC handler used
	// by proxycfg.
	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *structs.IndexedCheckServiceNodes, error) {
			authz, err := s.deps.ACLResolver.ResolveTokenAndDefaultMeta(req.Token, &req.EnterpriseMeta, nil)
			if err != nil {
				return 0, nil, err
			}

			idx, nodes, err := store.ServiceDump(ws, req.ServiceKind, req.UseServiceKind, &req.EnterpriseMeta, req.PeerName)
			if err != nil {
				return 0, nil, err
			}

			raw, err := filter.Execute(nodes)
			if err != nil {
				return 0, nil, fmt.Errorf("could not filter local service dump: %w", err)
			}
			nodes = raw.(structs.CheckServiceNodes)

			aclfilter.New(authz, s.deps.Logger).Filter(&nodes)

			return idx, &structs.IndexedCheckServiceNodes{
				Nodes: nodes,
				QueryMeta: structs.QueryMeta{
					Index:   idx,
					Backend: structs.QueryBackendBlocking,
				},
			}, nil
		},
		dispatchBlockingQueryUpdate[*structs.IndexedCheckServiceNodes](ch),
	)
}
