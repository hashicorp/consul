// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
)

// CacheIntentionUpstreams satisfies the proxycfg.IntentionUpstreams interface
// by sourcing upstreams for the given service, inferred from intentions, from
// the agent cache.
func CacheIntentionUpstreams(c *cache.Cache) proxycfg.IntentionUpstreams {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.IntentionUpstreamsName}
}

// CacheIntentionUpstreamsDestination satisfies the proxycfg.IntentionUpstreams
// interface by sourcing upstreams for the given destination, inferred from
// intentions, from the agent cache.
func CacheIntentionUpstreamsDestination(c *cache.Cache) proxycfg.IntentionUpstreams {
	return &cacheProxyDataSource[*structs.ServiceSpecificRequest]{c, cachetype.IntentionUpstreamsDestinationName}
}

// ServerIntentionUpstreams satisfies the proxycfg.IntentionUpstreams interface
// by sourcing upstreams for the given service, inferred from intentions, from
// the server's state store.
func ServerIntentionUpstreams(deps ServerDataSourceDeps) proxycfg.IntentionUpstreams {
	return serverIntentionUpstreams{deps, structs.IntentionTargetService}
}

// ServerIntentionUpstreamsDestination satisfies the proxycfg.IntentionUpstreams
// interface by sourcing upstreams for the given destination, inferred from
// intentions, from the server's state store.
func ServerIntentionUpstreamsDestination(deps ServerDataSourceDeps) proxycfg.IntentionUpstreams {
	return serverIntentionUpstreams{deps, structs.IntentionTargetDestination}
}

type serverIntentionUpstreams struct {
	deps   ServerDataSourceDeps
	target structs.IntentionTargetType
}

func (s serverIntentionUpstreams) Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	target := structs.NewServiceName(req.ServiceName, &req.EnterpriseMeta)

	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *structs.IndexedServiceList, error) {
			authz, err := s.deps.ACLResolver.ResolveTokenAndDefaultMeta(req.Token, &req.EnterpriseMeta, nil)
			if err != nil {
				return 0, nil, err
			}
			defaultDecision := authz.IntentionDefaultAllow(nil)

			index, services, err := store.IntentionTopology(ws, target, false, defaultDecision, s.target)
			if err != nil {
				return 0, nil, err
			}

			result := &structs.IndexedServiceList{
				Services: services,
				QueryMeta: structs.QueryMeta{
					Index:   index,
					Backend: structs.QueryBackendBlocking,
				},
			}
			aclfilter.New(authz, s.deps.Logger).Filter(result)

			return index, result, nil
		},
		dispatchBlockingQueryUpdate[*structs.IndexedServiceList](ch),
	)
}
