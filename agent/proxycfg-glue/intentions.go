// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfgglue

import (
	"context"
	"sort"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
)

// CacheIntentions satisfies the proxycfg.Intentions interface by sourcing data
// from the agent cache.
func CacheIntentions(c *cache.Cache) proxycfg.Intentions {
	return cacheIntentions{c}
}

type cacheIntentions struct {
	c *cache.Cache
}

func toIntentionMatchEntry(req *structs.ServiceSpecificRequest) structs.IntentionMatchEntry {
	return structs.IntentionMatchEntry{
		Partition: req.PartitionOrDefault(),
		Namespace: req.NamespaceOrDefault(),
		Name:      req.ServiceName,
	}
}

func (c cacheIntentions) Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	query := &structs.IntentionQueryRequest{
		Match: &structs.IntentionQueryMatch{
			Type:    structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{toIntentionMatchEntry(req)},
		},
		QueryOptions: structs.QueryOptions{Token: req.QueryOptions.Token},
	}
	return c.c.NotifyCallback(ctx, cachetype.IntentionMatchName, query, correlationID, func(ctx context.Context, event cache.UpdateEvent) {
		var result any
		if event.Err == nil {
			rsp, ok := event.Result.(*structs.IndexedIntentionMatches)
			if !ok {
				return
			}

			var matches structs.SimplifiedIntentions
			if len(rsp.Matches) != 0 {
				matches = structs.SimplifiedIntentions(rsp.Matches[0])
			}
			result = matches
		}

		select {
		case ch <- newUpdateEvent(correlationID, result, event.Err):
		case <-ctx.Done():
		}
	})
}

// ServerIntentions satisfies the proxycfg.Intentions interface by sourcing
// data from local materialized views (backed by EventPublisher subscriptions).
func ServerIntentions(deps ServerDataSourceDeps) proxycfg.Intentions {
	return &serverIntentions{deps}
}

type serverIntentions struct {
	deps ServerDataSourceDeps
}

func (s *serverIntentions) Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, structs.SimplifiedIntentions, error) {
			authz, err := s.deps.ACLResolver.ResolveTokenAndDefaultMeta(req.Token, &req.EnterpriseMeta, nil)
			if err != nil {
				return 0, nil, err
			}
			match := toIntentionMatchEntry(req)

			index, ixns, err := store.IntentionMatchOne(ws, match, structs.IntentionMatchDestination, structs.IntentionTargetService)
			if err != nil {
				return 0, nil, err
			}

			indexedIntentions := &structs.IndexedIntentions{
				Intentions: structs.Intentions(ixns),
			}

			aclfilter.New(authz, s.deps.Logger).Filter(indexedIntentions)

			sort.Sort(structs.IntentionPrecedenceSorter(indexedIntentions.Intentions))

			return index, structs.SimplifiedIntentions(indexedIntentions.Intentions), nil
		},
		dispatchBlockingQueryUpdate[structs.SimplifiedIntentions](ch),
	)
}
