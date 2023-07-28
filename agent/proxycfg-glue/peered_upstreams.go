package proxycfgglue

import (
	"context"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// CachePeeredUpstreams satisfies the proxycfg.PeeredUpstreams interface
// by sourcing data from the agent cache.
func CachePeeredUpstreams(c *cache.Cache) proxycfg.PeeredUpstreams {
	return &cacheProxyDataSource[*structs.PartitionSpecificRequest]{c, cachetype.PeeredUpstreamsName}
}

// ServerPeeredUpstreams satisfies the proxycfg.PeeredUpstreams interface by
// sourcing data from a blocking query against the server's state store.
func ServerPeeredUpstreams(deps ServerDataSourceDeps) proxycfg.PeeredUpstreams {
	return &serverPeeredUpstreams{deps}
}

type serverPeeredUpstreams struct {
	deps ServerDataSourceDeps
}

func (s *serverPeeredUpstreams) Notify(ctx context.Context, req *structs.PartitionSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *structs.IndexedPeeredServiceList, error) {
			var authzCtx acl.AuthorizerContext
			authz, err := s.deps.ACLResolver.ResolveTokenAndDefaultMeta(req.Token, &req.EnterpriseMeta, &authzCtx)
			if err != nil {
				return 0, nil, err
			}
			if err := authz.ToAllowAuthorizer().ServiceWriteAnyAllowed(&authzCtx); err != nil {
				return 0, nil, err
			}

			index, vips, err := store.VirtualIPsForAllImportedServices(ws, req.EnterpriseMeta)
			if err != nil {
				return 0, nil, err
			}

			result := make([]structs.PeeredServiceName, 0, len(vips))
			for _, vip := range vips {
				result = append(result, vip.Service)
			}

			return index, &structs.IndexedPeeredServiceList{
				Services: result,
				QueryMeta: structs.QueryMeta{
					Index:   index,
					Backend: structs.QueryBackendBlocking,
				},
			}, nil
		},
		dispatchBlockingQueryUpdate[*structs.IndexedPeeredServiceList](ch),
	)
}
