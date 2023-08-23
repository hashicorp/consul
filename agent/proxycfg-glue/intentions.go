package proxycfgglue

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// CacheIntentions satisfies the proxycfg.Intentions interface by sourcing data
// from the agent cache.
func CacheIntentions(c *cache.Cache) proxycfg.Intentions {
	return cacheIntentions{c}
}

type cacheIntentions struct {
	c *cache.Cache
}

func (c cacheIntentions) Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	query := &structs.IntentionQueryRequest{
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Partition: req.PartitionOrDefault(),
					Namespace: req.NamespaceOrDefault(),
					Name:      req.ServiceName,
				},
			},
		},
		QueryOptions: structs.QueryOptions{Token: req.QueryOptions.Token},
	}
	return c.c.NotifyCallback(ctx, cachetype.IntentionMatchName, query, correlationID, func(ctx context.Context, event cache.UpdateEvent) {
		e := proxycfg.UpdateEvent{
			CorrelationID: correlationID,
			Err:           event.Err,
		}

		if e.Err == nil {
			rsp, ok := event.Result.(*structs.IndexedIntentionMatches)
			if !ok {
				return
			}

			var matches structs.Intentions
			if len(rsp.Matches) != 0 {
				matches = rsp.Matches[0]
			}
			e.Result = matches
		}

		select {
		case ch <- e:
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
	// We may consume *multiple* streams (to handle wildcard intentions) and merge
	// them into a single list of intentions.
	//
	// An alternative approach would be to consume events for all intentions and
	// filter out the irrelevant ones. This would remove some complexity here but
	// at the expense of significant overhead.
	subjects := s.buildSubjects(req.ServiceName, req.EnterpriseMeta)

	// mu guards state, as the callback functions provided in NotifyCallback below
	// will be called in different goroutines.
	var mu sync.Mutex
	state := make([]*structs.ConfigEntryResponse, len(subjects))

	// buildEvent constructs an event containing the matching intentions received
	// from NotifyCallback calls below. If we have not received initial snapshots
	// for all streams yet, the event will be empty and the second return value will
	// be false (causing no event to be emittied).
	//
	// Note: mu must be held when calling this function.
	buildEvent := func() (proxycfg.UpdateEvent, bool) {
		intentions := make(structs.Intentions, 0)

		for _, result := range state {
			if result == nil {
				return proxycfg.UpdateEvent{}, false
			}
			si, ok := result.Entry.(*structs.ServiceIntentionsConfigEntry)
			if !ok {
				continue
			}
			intentions = append(intentions, si.ToIntentions()...)
		}

		sort.Sort(structs.IntentionPrecedenceSorter(intentions))

		return proxycfg.UpdateEvent{
			CorrelationID: correlationID,
			Result:        intentions,
		}, true
	}

	for subjectIdx, subject := range subjects {
		subjectIdx := subjectIdx

		storeReq := intentionsRequest{
			deps:    s.deps,
			baseReq: req,
			subject: subject,
		}
		err := s.deps.ViewStore.NotifyCallback(ctx, storeReq, correlationID, func(ctx context.Context, cacheEvent cache.UpdateEvent) {
			mu.Lock()
			state[subjectIdx] = cacheEvent.Result.(*structs.ConfigEntryResponse)
			event, ready := buildEvent()
			mu.Unlock()

			if ready {
				select {
				case ch <- event:
				case <-ctx.Done():
				}
			}

		})
		if err != nil {
			return err
		}
	}

	return nil
}

type intentionsRequest struct {
	deps    ServerDataSourceDeps
	baseReq *structs.ServiceSpecificRequest
	subject *pbsubscribe.NamedSubject
}

func (r intentionsRequest) CacheInfo() cache.RequestInfo {
	info := r.baseReq.CacheInfo()
	info.Key = fmt.Sprintf("%s/%s/%s/%s",
		r.subject.PeerName,
		r.subject.Partition,
		r.subject.Namespace,
		r.subject.Key,
	)
	return info
}

func (r intentionsRequest) NewMaterializer() (submatview.Materializer, error) {
	return submatview.NewLocalMaterializer(submatview.LocalMaterializerDeps{
		Backend:     r.deps.EventPublisher,
		ACLResolver: r.deps.ACLResolver,
		Deps: submatview.Deps{
			View:    &configEntryView{},
			Logger:  r.deps.Logger,
			Request: r.Request,
		},
	}), nil
}

func (r intentionsRequest) Request(index uint64) *pbsubscribe.SubscribeRequest {
	return &pbsubscribe.SubscribeRequest{
		Topic:      pbsubscribe.Topic_ServiceIntentions,
		Index:      index,
		Datacenter: r.baseReq.Datacenter,
		Token:      r.baseReq.Token,
		Subject:    &pbsubscribe.SubscribeRequest_NamedSubject{NamedSubject: r.subject},
	}
}

func (r intentionsRequest) Type() string { return "proxycfgglue.ServiceIntentions" }
