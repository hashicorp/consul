package upgrade

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

const retryTimeout = 20 * time.Second
const retryFrequency = 500 * time.Millisecond

func LongFailer() *retry.Timer {
	return &retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}
}

func waitForLeader(t *testing.T, Cluster *cluster.Cluster) {
	retry.RunWith(LongFailer(), t, func(r *retry.R) {
		leader, err := Cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
	})
}

func waitForMembers(t *testing.T, client *api.Client, expectN int) {
	retry.RunWith(LongFailer(), t, func(r *retry.R) {
		members, err := client.Agent().Members(false)
		require.NoError(r, err)
		require.Len(r, members, expectN)
	})
}
