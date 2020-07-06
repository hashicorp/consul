package state

import (
	"github.com/hashicorp/consul/agent/consul/stream"
)

// newTopicHandlers returns the default handlers for state change events.
func newTopicHandlers() map[stream.Topic]TopicHandler {
	return map[stream.Topic]TopicHandler{
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
