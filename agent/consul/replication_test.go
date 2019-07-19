package consul

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplicationRestart(t *testing.T) {
	config := ReplicatorConfig{
		Name: "mock",
		ReplicateFn: func(ctx context.Context, lastRemoteIndex uint64) (uint64, bool, error) {
			return 1, false, nil
		},
		Rate:  1,
		Burst: 1,
	}

	repl, err := NewReplicator(&config)
	require.NoError(t, err)

	repl.Start()
	repl.Stop()
	repl.Start()
	// Previously this would have segfaulted
	repl.Stop()
}
