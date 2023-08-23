package proxycfgglue

import (
	"context"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
)

// ServerIntentionUpstreams satisfies the proxycfg.IntentionUpstreams interface
// by sourcing data from a blocking query against the server's state store.
func ServerIntentionUpstreams(deps ServerDataSourceDeps) proxycfg.IntentionUpstreams {
	return serverIntentionUpstreams{deps}
}

type serverIntentionUpstreams struct {
	deps ServerDataSourceDeps
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

			index, services, err := store.IntentionTopology(ws, target, false, defaultDecision, structs.IntentionTargetService)
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

func dispatchBlockingQueryUpdate[ResultType any](ch chan<- proxycfg.UpdateEvent) func(context.Context, string, ResultType, error) {
	return func(ctx context.Context, correlationID string, result ResultType, err error) {
		event := proxycfg.UpdateEvent{
			CorrelationID: correlationID,
			Result:        result,
			Err:           err,
		}
		select {
		case ch <- event:
		case <-ctx.Done():
		}
	}
}
