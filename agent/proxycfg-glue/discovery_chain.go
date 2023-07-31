package proxycfgglue

import (
	"context"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// CacheCompiledDiscoveryChain satisfies the proxycfg.CompiledDiscoveryChain
// interface by sourcing data from the agent cache.
func CacheCompiledDiscoveryChain(c *cache.Cache) proxycfg.CompiledDiscoveryChain {
	return &cacheProxyDataSource[*structs.DiscoveryChainRequest]{c, cachetype.CompiledDiscoveryChainName}
}

// ServerCompiledDiscoveryChain satisfies the proxycfg.CompiledDiscoveryChain
// interface by sourcing data from a blocking query against the server's state
// store.
//
// Requests for services in remote datacenters will be delegated to the given
// remoteSource (i.e. CacheCompiledDiscoveryChain).
func ServerCompiledDiscoveryChain(deps ServerDataSourceDeps, remoteSource proxycfg.CompiledDiscoveryChain) proxycfg.CompiledDiscoveryChain {
	return &serverCompiledDiscoveryChain{deps, remoteSource}
}

type serverCompiledDiscoveryChain struct {
	deps         ServerDataSourceDeps
	remoteSource proxycfg.CompiledDiscoveryChain
}

func (s serverCompiledDiscoveryChain) Notify(ctx context.Context, req *structs.DiscoveryChainRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	if req.Datacenter != s.deps.Datacenter {
		return s.remoteSource.Notify(ctx, req, correlationID, ch)
	}

	entMeta := req.GetEnterpriseMeta()

	evalDC := req.EvaluateInDatacenter
	if evalDC == "" {
		evalDC = s.deps.Datacenter
	}

	compileReq := discoverychain.CompileRequest{
		ServiceName:            req.Name,
		EvaluateInNamespace:    entMeta.NamespaceOrDefault(),
		EvaluateInPartition:    entMeta.PartitionOrDefault(),
		EvaluateInDatacenter:   evalDC,
		OverrideMeshGateway:    req.OverrideMeshGateway,
		OverrideProtocol:       req.OverrideProtocol,
		OverrideConnectTimeout: req.OverrideConnectTimeout,
	}

	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *structs.DiscoveryChainResponse, error) {
			var authzContext acl.AuthorizerContext
			authz, err := s.deps.ACLResolver.ResolveTokenAndDefaultMeta(req.Token, req.GetEnterpriseMeta(), &authzContext)
			if err != nil {
				return 0, nil, err
			}
			if err := authz.ToAllowAuthorizer().ServiceReadAllowed(req.Name, &authzContext); err != nil {
				// TODO(agentless): the agent cache handles acl.IsErrNotFound specially to
				// prevent endlessly retrying if an ACL token is deleted. We should probably
				// do this in watch.ServerLocalNotify too.
				return 0, nil, err
			}

			index, chain, entries, err := store.ServiceDiscoveryChain(ws, req.Name, entMeta, compileReq)
			if err != nil {
				return 0, nil, err
			}

			rsp := &structs.DiscoveryChainResponse{
				Chain: chain,
				QueryMeta: structs.QueryMeta{
					Backend: structs.QueryBackendBlocking,
					Index:   index,
				},
			}

			// TODO(boxofrad): Check with @mkeeler that this is the correct thing to do.
			if entries.IsEmpty() {
				return index, rsp, watch.ErrorNotFound
			}
			return index, rsp, nil
		},
		dispatchBlockingQueryUpdate[*structs.DiscoveryChainResponse](ch),
	)
}
