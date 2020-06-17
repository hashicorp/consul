package state

import (
	"github.com/hashicorp/consul/agent/consul/stream"
	memdb "github.com/hashicorp/go-memdb"
)

// topicHandler provides functions which create stream.Events for a topic.
type topicHandler struct {
	// Snapshot creates the necessary events to reproduce the current state and
	// appends them to the EventBuffer.
	Snapshot func(*stream.SubscribeRequest, *stream.EventBuffer) (index uint64, err error)
	// ProcessChanges accepts a slice of Changes, and builds a slice of events for
	// those changes.
	ProcessChanges func(*txn, memdb.Changes) ([]stream.Event, error)
}

// newTopicHandlers returns the default handlers for state change events.
func newTopicHandlers() map[stream.Topic]topicHandler {
	return map[stream.Topic]topicHandler{
		// For now we don't actually support subscribing to ACL* topics externally
		// so these have no Snapshot methods yet. We do need to have a
		// ProcessChanges func to publish the partial events on ACL changes though
		// so that we can invalidate other subscriptions if their effective ACL
		// permissions change.
		stream.Topic_ACLTokens: {
			ProcessChanges: aclEventsFromChanges,
		},
		// Note no ACLPolicies/ACLRoles defined yet because we publish all events
		// from one handler to save on iterating/filtering and duplicating code and
		// there are no snapshots for these yet per comment above.
	}
}
