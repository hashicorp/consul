package state

import (
	"github.com/hashicorp/consul/agent/consul/stream"
)

// newTopicHandlers returns the default handlers for state change events.
func newTopicHandlers() map[stream.Topic]TopicHandler {
	return map[stream.Topic]TopicHandler{
		// TopicInternal is a special case for processors that handle events that are
		// not for subscribers. They are used by the stream package.
		stream.TopicInternal: {ProcessChanges: aclChangeUnsubscribeEvent},
	}
}
