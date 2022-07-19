package proxycfgglue

import (
	"context"
	"errors"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// CacheTrustBundle satisfies the proxycfg.TrustBundle interface by sourcing
// data from the agent cache.
func CacheTrustBundle(c *cache.Cache) proxycfg.TrustBundle {
	return &cacheProxyDataSource[*pbpeering.TrustBundleReadRequest]{c, cachetype.TrustBundleReadName}
}

// ServerTrustBundle satisfies the proxycfg.TrustBundle interface by sourcing
// data from a blocking query against the server's state store.
func ServerTrustBundle(deps ServerDataSourceDeps) proxycfg.TrustBundle {
	return &serverTrustBundle{deps}
}

type serverTrustBundle struct {
	deps ServerDataSourceDeps
}

func (s *serverTrustBundle) Notify(ctx context.Context, req *pbpeering.TrustBundleReadRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	// TODO(peering): ACL check.
	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *pbpeering.TrustBundleReadResponse, error) {
			index, bundle, err := store.PeeringTrustBundleRead(ws, state.Query{
				Value:          req.Name,
				EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition),
			})
			if err != nil {
				return 0, nil, err
			}
			return index, &pbpeering.TrustBundleReadResponse{
				Index:  index,
				Bundle: bundle,
			}, nil
		},
		dispatchBlockingQueryUpdate[*pbpeering.TrustBundleReadResponse](ch),
	)
}

// CacheTrustBundleList satisfies the proxycfg.TrustBundleList interface by sourcing
// data from the agent cache.
func CacheTrustBundleList(c *cache.Cache) proxycfg.TrustBundleList {
	return &cacheProxyDataSource[*pbpeering.TrustBundleListByServiceRequest]{c, cachetype.TrustBundleListName}
}

// ServerTrustBundleList satisfies the proxycfg.TrustBundle interface by
// sourcing data from a blocking query against the server's state store.
func ServerTrustBundleList(deps ServerDataSourceDeps) proxycfg.TrustBundleList {
	return &serverTrustBundleList{deps}
}

type serverTrustBundleList struct {
	deps ServerDataSourceDeps
}

func (s *serverTrustBundleList) Notify(ctx context.Context, req *pbpeering.TrustBundleListByServiceRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	entMeta := acl.NewEnterpriseMetaWithPartition(req.Partition, req.Namespace)

	// TODO(peering): ACL check.
	return watch.ServerLocalNotify(ctx, correlationID, s.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *pbpeering.TrustBundleListByServiceResponse, error) {
			var (
				index   uint64
				bundles []*pbpeering.PeeringTrustBundle
				err     error
			)
			switch {
			case req.ServiceName != "":
				index, bundles, err = store.TrustBundleListByService(ws, req.ServiceName, s.deps.Datacenter, entMeta)
			case req.Kind == string(structs.ServiceKindMeshGateway):
				index, bundles, err = store.PeeringTrustBundleList(ws, entMeta)
			case req.Kind != "":
				err = errors.New("kind must be mesh-gateway if set")
			default:
				err = errors.New("one of service or kind is required")
			}
			if err != nil {
				return 0, nil, err
			}

			return index, &pbpeering.TrustBundleListByServiceResponse{
				Index:   index,
				Bundles: bundles,
			}, nil
		},
		dispatchBlockingQueryUpdate[*pbpeering.TrustBundleListByServiceResponse](ch),
	)
}
