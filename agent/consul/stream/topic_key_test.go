package stream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTopicKey(t *testing.T) {
	require.Equal(t,
		NewTopicKey("foo", "bar", "baz"),
		NewTopicKey("FOO", "BAR", "BAZ"),
		"case differences should be ignored",
	)

	require.Equal(t,
		NewTopicKey("foo", "", ""),
		NewTopicKey("foo", "default", "default"),
		"empty namespace/partition should be treated as the default namespace/partition",
	)

	require.NotEqual(t,
		NewTopicKey("foo", "bar", "baz"),
		NewTopicKey("qux", "bar", "baz"),
		"different keys should be treated differently",
	)

	require.NotEqual(t,
		NewTopicKey("foo", "bar", "baz"),
		NewTopicKey("foo", "qux", "baz"),
		"different namespaces should be treated differently",
	)

	require.NotEqual(t,
		NewTopicKey("foo", "bar", "baz"),
		NewTopicKey("foo", "bar", "qux"),
		"different partitions should be treated differently",
	)
}
