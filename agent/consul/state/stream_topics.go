package state

import (
	"github.com/hashicorp/consul/agent/consul/stream"
	memdb "github.com/hashicorp/go-memdb"
)

// unboundSnapFn is a stream.SnapFn with state store as the first argument. This
// is bound to a concrete state store instance in the EventPublisher on startup.
type unboundSnapFn func(*Store, *stream.SubscribeRequest, *stream.EventBuffer) (uint64, error)
type unboundProcessChangesFn func(*Store, *txn, memdb.Changes) ([]stream.Event, error)

// topicHandlers describes the methods needed to process a streaming
// subscription for a given topic.
type topicHandlers struct {
	Snapshot       unboundSnapFn
	ProcessChanges unboundProcessChangesFn
}

// topicRegistry is a map of topic handlers. It must only be written to during
// init().
var topicRegistry map[stream.Topic]topicHandlers

func init() {
	topicRegistry = map[stream.Topic]topicHandlers{
		stream.Topic_ServiceHealth: topicHandlers{
			Snapshot:       (*Store).ServiceHealthSnapshot,
			ProcessChanges: (*Store).ServiceHealthEventsFromChanges,
		},
		stream.Topic_ServiceHealthConnect: topicHandlers{
			Snapshot: (*Store).ServiceHealthConnectSnapshot,
			// Note there is no ProcessChanges since Connect events are published by
			// the same event publisher as regular health events to avoid duplicating
			// lots of filtering on every commit.
		},
		// For now we don't actually support subscribing to ACL* topics externally
		// so these have no Snapshot methods yet. We do need to have a
		// ProcessChanges func to publish the partial events on ACL changes though
		// so that we can invalidate other subscriptions if their effective ACL
		// permissions change.
		stream.Topic_ACLTokens: topicHandlers{
			ProcessChanges: aclEventsFromChanges,
		},
		// Note no ACLPolicies/ACLRoles defined yet because we publish all events
		// from one handler to save on iterating/filtering and duplicating code and
		// there are no snapshots for these yet per comment above.
	}
}
