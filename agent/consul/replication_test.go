package consul

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestReplicationRestart(t *testing.T) {
	mgr := NewLeaderRoutineManager(testutil.TestLogger(t))

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

	mgr.Start("mock", repl.Run)
	mgr.Stop("mock")
	mgr.Start("mock", repl.Run)
	// Previously this would have segfaulted
	mgr.Stop("mock")
}
