// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"time"

	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

func (a *Agent) runClientFeatureGateCache(store *featuregate.Store, index uint64) {
	const retryInterval = 5 * time.Second
	// Older servers reject Operator.FeatureGateGet. Retry slowly so a later
	// upgrade is noticed without spamming both sides every few seconds.
	const unavailableRetryInterval = 5 * time.Minute
	unavailableLogged := false
	for {
		select {
		case <-a.shutdownCh:
			return
		default:
		}

		var err error
		index, err = a.refreshClientFeatureGates(store, index, true)
		if err == nil && store.Current().StatusIndex > 0 {
			unavailableLogged = false
			continue
		}

		wait := retryInterval
		if err != nil {
			if isFeatureGateUnavailable(err) {
				if !unavailableLogged {
					a.logger.Info("feature gate RPCs unavailable on servers; using local defaults")
					unavailableLogged = true
				}
				wait = unavailableRetryInterval
			} else {
				a.logger.Warn("failed to refresh client feature gates", "error", err)
			}
		}
		select {
		case <-a.shutdownCh:
			return
		case <-time.After(wait):
		}
	}
}

func (a *Agent) refreshClientFeatureGates(store *featuregate.Store, index uint64, blocking bool) (uint64, error) {
	// Stale reads let any server answer, instead of every client's minute-long
	// poll concentrating on the leader.
	queryOptions := structs.QueryOptions{Token: a.tokens.AgentToken(), AllowStale: true}
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
		// A lagging follower may predate the RPC while the leader already
		// supports it; try the leader once before treating the cluster as old.
		if isFeatureGateUnavailable(err) && request.AllowStale {
			request.AllowStale = false
			request.MinQueryIndex = 0
			if err2 := a.RPC(ctx, "Operator.FeatureGateGet", request, &reply); err2 != nil {
				return index, err2
			}
		} else {
			return index, err
		}
	}
	publishClientFeatureGates(store, &reply)
	// Block on the newest generation installed, so a lagging server waits to
	// catch up rather than handing back an older one.
	return store.Current().StatusIndex, nil
}

// publishClientFeatureGates installs reply only when it is newer than the cached
// generation, so a lagging follower cannot roll the cache back.
func publishClientFeatureGates(store *featuregate.Store, reply *structs.FeatureGateQueryResponse) {
	if reply.Uninitialized || len(reply.Features) == 0 {
		// Only the leader (LastContact == 0) is authoritative that no gate state
		// exists; a follower may simply not have replayed it yet.
		if reply.LastContact == 0 {
			store.Reset(0)
		}
		return
	}
	features := make(map[string]bool, len(reply.Features))
	for _, feature := range reply.Features {
		features[feature.Name] = feature.EffectiveEnabled
	}
	store.Publish(featuregate.Snapshot{StatusIndex: reply.Index, Features: features})
}
