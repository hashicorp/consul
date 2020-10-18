package autopilot

import (
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
)

func TestPromotion(t *testing.T) {
	config := &Config{
		LastContactThreshold:    5 * time.Second,
		MaxTrailingLogs:         100,
		ServerStabilizationTime: 3 * time.Second,
	}

	cases := []struct {
		name       string
		conf       *Config
		health     OperatorHealthReply
		servers    []raft.Server
		promotions []raft.Server
	}{
		{
			name: "one stable voter, no promotions",
			conf: config,
			health: OperatorHealthReply{
				Servers: []ServerHealth{
					{
						ID:          "a",
						Healthy:     true,
						StableSince: time.Now().Add(-10 * time.Second),
					},
				},
			},
			servers: []raft.Server{
				{ID: "a", Suffrage: raft.Voter},
			},
		},
		{
			name: "one stable nonvoter, should be promoted",
			conf: config,
			health: OperatorHealthReply{
				Servers: []ServerHealth{
					{
						ID:          "a",
						Healthy:     true,
						StableSince: time.Now().Add(-10 * time.Second),
					},
					{
						ID:          "b",
						Healthy:     true,
						StableSince: time.Now().Add(-10 * time.Second),
					},
				},
			},
			servers: []raft.Server{
				{ID: "a", Suffrage: raft.Voter},
				{ID: "b", Suffrage: raft.Nonvoter},
			},
			promotions: []raft.Server{
				{ID: "b", Suffrage: raft.Nonvoter},
			},
		},
		{
			name: "unstable servers, neither should be promoted",
			conf: config,
			health: OperatorHealthReply{
				Servers: []ServerHealth{
					{
						ID:          "a",
						Healthy:     true,
						StableSince: time.Now().Add(-10 * time.Second),
					},
					{
						ID:          "b",
						Healthy:     false,
						StableSince: time.Now().Add(-10 * time.Second),
					},
					{
						ID:          "c",
						Healthy:     true,
						StableSince: time.Now().Add(-1 * time.Second),
					},
				},
			},
			servers: []raft.Server{
				{ID: "a", Suffrage: raft.Voter},
				{ID: "b", Suffrage: raft.Nonvoter},
				{ID: "c", Suffrage: raft.Nonvoter},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			promotions := PromoteStableServers(tc.conf, tc.health, tc.servers)
			require.Equal(t, tc.promotions, promotions)
		})
	}
}
