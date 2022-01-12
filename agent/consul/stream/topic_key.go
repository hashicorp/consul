package stream

import "strings"

// TopicKey identifies a portion of a topic for which a subscriber wishes to
// receive events (e.g. health events for a particular service). It is used as
// a map key when storing/retrieving event buffers and cached snapshots.
//
// Note: its fields are unexported as it's expected that users will call
// NewTopicKey rather than constructing a TopicKey manually, as this method
// normalizes the given values.
type TopicKey struct {
	key       string
	namespace string
	partition string
}

// NewTopicKey constructs a TopicKey for the given {key, namespace, partition}
// combination. The given values will be normalized - converted to lower-case
// with empty namespace/partition treated as the default namespace/partition -
// to ensure equality when using the TopicKey as a map key.
func NewTopicKey(key, namespace, partition string) TopicKey {
	if namespace == "" {
		namespace = "default"
	}

	if partition == "" {
		partition = "default"
	}

	return TopicKey{
		key:       strings.ToLower(key),
		namespace: strings.ToLower(namespace),
		partition: strings.ToLower(partition),
	}
}
