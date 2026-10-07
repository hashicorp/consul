package agent

import (
	"context"
	"time"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

func (a *Agent) runClientFeatureGateCache(store *featuregate.Store, index uint64) {
	const retryInterval = 5 * time.Second
	for {
		select {
		case <-a.shutdownCh:
			return
		default:
		}

		var err error
		index, err = a.refreshClientFeatureGates(store, index, true)
		if err == nil && store.Current().StatusIndex > 0 {
			continue
		}

		if err != nil {
			a.logger.Warn("failed to refresh client feature gates", "error", err)
		}
		select {
		case <-a.shutdownCh:
			return
		case <-time.After(retryInterval):
		}
	}
}

func (a *Agent) refreshClientFeatureGates(store *featuregate.Store, index uint64, blocking bool) (uint64, error) {
	queryOptions := structs.QueryOptions{Token: a.tokens.AgentToken()}
	if blocking {
		queryOptions.MinQueryIndex = index
		queryOptions.MaxQueryTime = time.Minute
	}
	request := &structs.FeatureGateQueryRequest{
		Node: a.config.NodeName,
		DCSpecificRequest: structs.DCSpecificRequest{
			Datacenter:     a.config.Datacenter,
			EnterpriseMeta: *a.AgentEnterpriseMeta(),
			QueryOptions:   queryOptions,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute+10*time.Second)
	defer cancel()

	var reply structs.FeatureGateQueryResponse
	if err := a.RPC(ctx, "Operator.FeatureGateGet", request, &reply); err != nil {
		return index, err
	}
	publishClientFeatureGates(store, &reply)
	if reply.Uninitialized || len(reply.Features) == 0 {
		return 0, nil
	}
	return reply.Index, nil
}

func publishClientFeatureGates(store *featuregate.Store, reply *structs.FeatureGateQueryResponse) {
	if reply.Uninitialized || len(reply.Features) == 0 {
		store.Reset(0)
		return
	}
	if reply.Index < store.Current().StatusIndex {
		store.Reset(0)
	}
	features := make(map[string]bool, len(reply.Features))
	for _, feature := range reply.Features {
		features[feature.Name] = feature.EffectiveEnabled
	}
	store.Publish(featuregate.Snapshot{StatusIndex: reply.Index, Features: features})
}
