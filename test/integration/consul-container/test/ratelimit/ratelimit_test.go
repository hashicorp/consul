package ratelimit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const (
	retryableErrorMsg    = "Unexpected response code: 429 (rate limit exceeded, try again later or against a different server)"
	nonRetryableErrorMsg = "Unexpected response code: 503 (rate limit exceeded for operation that can only be performed by the leader, try again later)"
)

// TestRateLimit
// This test validates
// - enforcing mode
//   - read_rate - returns 429 - was blocked and returns retryable error
//   - write_rate - returns 503 - was blocked and is not retryable
//   - on each
//     - fires metrics forexceeding
//     - logs for exceeding

func TestRateLimit(t *testing.T) {
	type action struct {
		function           func(client *api.Client) error
		httpAction         string
		rateLimitOperation string
		rateLimitType      string // will become an array of strings
	}
	type operation struct {
		action           action
		expectedErrorMsg string
		expectLog        bool
		expectMetric     bool
	}
	type testCase struct {
		description string
		cmd         string
		operations  []operation
	}

	getNodes := action{
		function: func(client *api.Client) error {
			_, _, err := client.Catalog().Nodes(&api.QueryOptions{})
			return err
		},
		httpAction:         "method=GET url=/v1/catalog/nodes",
		rateLimitOperation: "Catalog.ListNodes",
		rateLimitType:      "global/read",
	}
	putConfig := action{
		function: func(client *api.Client) error {
			_, err := utils.ApplyDefaultProxySettings(client)
			return err
		},
		httpAction:         "method=PUT url=/v1/config",
		rateLimitOperation: "ConfigEntry.Apply",
		rateLimitType:      "global/write",
	}

	testCases := []testCase{
		{
			description: "Mode: disabled - errors: no / logs: no / metrics: no",
			cmd:         "-hcl=limits { request_limits { mode = \"disabled\" read_rate = 0 write_rate = 0 }}",
			operations: []operation{
				{
					action:           getNodes,
					expectedErrorMsg: "",
					expectLog:        false,
					expectMetric:     false,
				},
				{
					action:           putConfig,
					expectedErrorMsg: "",
					expectLog:        false,
					expectMetric:     false,
				},
			},
		},
		{
			description: "Mode: permissive - errors: no / logs: no / metrics: yes",
			cmd:         "-hcl=limits { request_limits { mode = \"permissive\" read_rate = 0 write_rate = 0 }}",
			operations: []operation{
				{
					action:           getNodes,
					expectedErrorMsg: "",
					expectLog:        false,
					expectMetric:     false,
				},
				{
					action:           putConfig,
					expectedErrorMsg: "",
					expectLog:        false,
					expectMetric:     false,
				},
			},
		},
		{
			description: "Mode: enforcing - errors: yes / logs: yes / metrics: yes",
			cmd:         "-hcl=limits { request_limits { mode = \"enforcing\" read_rate = 0 write_rate = 0 }}",
			operations: []operation{
				{
					action:           getNodes,
					expectedErrorMsg: retryableErrorMsg,
					expectLog:        true,
					expectMetric:     true,
				},
				{
					action:           putConfig,
					expectedErrorMsg: nonRetryableErrorMsg,
					expectLog:        true,
					expectMetric:     true,
				},
			},
		}}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			urlsExpectingLogging := []string{}
			logConsumer := &TestLogConsumer{}
			cluster := createCluster(t, tc.cmd, logConsumer)
			defer terminate(t, cluster)

			client, err := cluster.GetClient(nil, true)
			require.NoError(t, err)

			// validate returned errors to client
			for _, op := range tc.operations {
				if op.expectLog {
					urlsExpectingLogging = append(urlsExpectingLogging, op.action.httpAction)
				}

				err = op.action.function(client)
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Equal(t, op.expectedErrorMsg, err.Error())
				} else {
					require.NoError(t, err)
				}
			}

			// validate logs
			for _, httpRequest := range urlsExpectingLogging {
				found := false
				for _, msg := range logConsumer.Msgs {
					if strings.Contains(msg, httpRequest) {
						found = true
					}
				}
				require.True(t, found, fmt.Sprintf("Log not found for: %s", httpRequest))
			}

			// validate metrics
			metricInfo, err := client.Agent().Metrics()
			// TODO(NET-1978): currently returns NaN error
			//			require.NoError(t, err)
			if metricInfo != nil {
				for _, op := range tc.operations {
					if op.expectMetric {
						for _, counter := range metricInfo.Counters {
							if counter.Name == "consul.rate.limit" {
								operation, ok := counter.Labels["op"]
								require.True(t, ok)

								limitType, ok := counter.Labels["limit_type"]
								require.True(t, ok)

								mode, ok := counter.Labels["mode"]
								require.True(t, ok)

								if operation == op.action.rateLimitOperation {
									require.Equal(t, 2, counter.Count)
									require.Equal(t, op.action.rateLimitType, limitType)
									require.Equal(t, "disabled", mode)
								}
							}
						}
					}
				}
			}
		})
	}
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

type TestLogConsumer struct {
	Msgs []string
}

func (g *TestLogConsumer) Accept(l testcontainers.Log) {
	g.Msgs = append(g.Msgs, string(l.Content))
}

// createCluster
func createCluster(t *testing.T, cmd string, logConsumer *TestLogConsumer) *libcluster.Cluster {
	opts := libcluster.BuildOptions{
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	conf := libcluster.NewConfigBuilder(ctx).ToAgentConfig(t)
	conf.LogConsumer = logConsumer

	t.Logf("Cluster config:\n%s", conf.JSON)

	parsedConfigs := []libcluster.Config{*conf}

	cfgs := []libcluster.Config{}
	for _, cfg := range parsedConfigs {
		cfg.Cmd = append(cfg.Cmd, cmd)
		cfgs = append(cfgs, cfg)
	}
	cluster, err := libcluster.New(t, cfgs)
	require.NoError(t, err)

	client, err := cluster.GetClient(nil, true)

	require.NoError(t, err)
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 1)

	return cluster
}
