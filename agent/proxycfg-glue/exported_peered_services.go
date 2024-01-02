// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"

	"github.com/hashicorp/consul/agent/structs/aclfilter"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// CacheExportedPeeredServices satisfies the proxycfg.ExportedPeeredServices
// interface by sourcing data from the agent cache.
func CacheExportedPeeredServices(c *cache.Cache) proxycfg.ExportedPeeredServices {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.ExportedPeeredServicesName}
}

// ServerExportedPeeredServices satisifies the proxycfg.ExportedPeeredServices
// interface by sourcing data from a blocking query against the server's state
// store.
func ServerExportedPeeredServices(deps ServerDataSourceDeps) proxycfg.ExportedPeeredServices {
	return &serverExportedPeeredServices{deps}
}

type serverExportedPeeredServices struct {
	deps ServerDataSourceDeps
}

func (s *serverExportedPeeredServices) Notify(ctx context.Context, req *structs.DCSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *structs.IndexedExportedServiceList, error) {
			authz, err := s.deps.ACLResolver.ResolveTokenAndDefaultMeta(req.Token, &req.EnterpriseMeta, nil)
			if err != nil {
				return 0, nil, err
			}

			index, serviceMap, err := store.ExportedServicesForAllPeersByName(ws, req.Datacenter, req.EnterpriseMeta)
			if err != nil {
				return 0, nil, err
			}

			result := &structs.IndexedExportedServiceList{
				Services: serviceMap,
				QueryMeta: structs.QueryMeta{
					Backend: structs.QueryBackendBlocking,
					Index:   index,
				},
			}
			aclfilter.New(authz, s.deps.Logger).Filter(result)

			return index, result, nil
		},
		dispatchBlockingQueryUpdate[*structs.IndexedExportedServiceList](ch),
	)
}
