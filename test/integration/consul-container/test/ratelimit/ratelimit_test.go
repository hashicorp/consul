package ratelimit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
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
		rateLimitOperation string
		rateLimitType      string // will become an array of strings
	}
	type operation struct {
		action            action
		expectedErrorMsg  string
		expectExceededLog bool
		expectBlockedLog  bool
		expectMetric      bool
	}
	type testCase struct {
		description string
		cmd         string
		operations  []operation
	}

	getKV := action{
		function: func(client *api.Client) error {
			_, _, err := client.KV().Get("foo", &api.QueryOptions{})
			return err
		},
		rateLimitOperation: "KVS.Get",
		rateLimitType:      "global/read",
	}
	putKV := action{
		function: func(client *api.Client) error {
			_, err := client.KV().Put(&api.KVPair{Key: "foo", Value: []byte("bar")}, &api.WriteOptions{})
			return err
		},
		rateLimitOperation: "KVS.Apply",
		rateLimitType:      "global/write",
	}

	testCases := []testCase{
		// HTTP & net/RPC
		{
			description: "HTTP & net/RPC / Mode: disabled - errors: no / logs: no / metrics: no",
			cmd:         "-hcl=limits { request_limits { mode = \"disabled\" read_rate = 0 write_rate = 0 }}",
			operations: []operation{
				{
					action:            putKV,
					expectedErrorMsg:  "",
					expectExceededLog: false,
					expectBlockedLog:  false,
					expectMetric:      false,
				},
				{
					action:            getKV,
					expectedErrorMsg:  "",
					expectExceededLog: false,
					expectBlockedLog:  false,
					expectMetric:      false,
				},
			},
		},
		{
			description: "HTTP & net/RPC / Mode: permissive - errors: no / logs: no / metrics: yes",
			cmd:         "-hcl=limits { request_limits { mode = \"permissive\" read_rate = 0 write_rate = 0 }}",
			operations: []operation{
				{
					action:            putKV,
					expectedErrorMsg:  "",
					expectExceededLog: true,
					expectBlockedLog:  false,
					expectMetric:      false,
				},
				{
					action:            getKV,
					expectedErrorMsg:  "",
					expectExceededLog: true,
					expectBlockedLog:  false,
					expectMetric:      false,
				},
			},
		},
		{
			description: "HTTP & net/RPC / Mode: enforcing - errors: yes / logs: yes / metrics: yes",
			cmd:         "-hcl=limits { request_limits { mode = \"enforcing\" read_rate = 0 write_rate = 0 }}",
			operations: []operation{
				{
					action:            putKV,
					expectedErrorMsg:  nonRetryableErrorMsg,
					expectExceededLog: true,
					expectBlockedLog:  true,
					expectMetric:      true,
				},
				{
					action:            getKV,
					expectedErrorMsg:  retryableErrorMsg,
					expectExceededLog: true,
					expectBlockedLog:  true,
					expectMetric:      true,
				},
			},
		}}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			logConsumer := &TestLogConsumer{}
			//using trace to be able to assert that rate exceeded messages appear.
			const logLvl = "TRACE"
			cluster := createCluster(t, tc.cmd, logConsumer, logLvl)
			defer terminate(t, cluster)

			client, err := cluster.GetClient(nil, true)
			require.NoError(t, err)

			// perform actions and validate returned errors to client
			for _, op := range tc.operations {
				err = op.action.function(client)
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Equal(t, op.expectedErrorMsg, err.Error())
				} else {
					require.NoError(t, err)
				}
			}

			// validate logs and metrics
			// doing this in a separate loop so we can perform actions, allow metrics
			// and logs to collect and then assert on each.
			for _, op := range tc.operations {
				// validate metrics
				metricsInfo, err := client.Agent().Metrics()
				// TODO(NET-1978): currently returns NaN error
				//			require.NoError(t, err)
				if metricsInfo != nil && err == nil {
					if op.expectMetric {
						checkForMetric(t, metricsInfo, op.action.rateLimitOperation, op.action.rateLimitType)
					}
				}

				// validate logs
				// putting this last as there are cases where logs
				// were not present in consumer when assertion was made.
				if op.expectExceededLog {
					checkLogsForMessage(t, logConsumer.Msgs, fmt.Sprintf("[TRACE] agent.server.rpc-rate-limit: RPC exceeded allowed rate limit: rpc=%s", op.action.rateLimitOperation),
						op.action.rateLimitOperation, "exceeded")
				}
				if op.expectBlockedLog {
					checkLogsForMessage(t, logConsumer.Msgs, fmt.Sprintf("[WARN]  agent.server.rpc-rate-limit: RPC blocked due to exceeded allowed rate limit: rpc=%s", op.action.rateLimitOperation),
						op.action.rateLimitOperation, "blocked")
				}
			}
		})
	}
}

func checkForMetric(t *testing.T, metricsInfo *api.MetricsInfo, operationName string, expectedLimitType string) {
	for _, counter := range metricsInfo.Counters {
		if counter.Name == "consul.rate.limit" {
			operation, ok := counter.Labels["op"]
			require.True(t, ok)

			limitType, ok := counter.Labels["limit_type"]
			require.True(t, ok)

			mode, ok := counter.Labels["mode"]
			require.True(t, ok)

			if operation == operationName {
				require.Equal(t, 2, counter.Count)
				require.Equal(t, expectedLimitType, limitType)
				require.Equal(t, "disabled", mode)
			}
		}
	}

}

func checkLogsForMessage(t *testing.T, logs []string, msg string, operationName string, logType string) {
	found := false
	for _, log := range logs {
		if strings.Contains(log, msg) {
			found = true
		}
	}
	require.True(t, found, fmt.Sprintf("%s log not found for: %s", logType, operationName))
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
func createCluster(t *testing.T, cmd string, logConsumer *TestLogConsumer, logLevel string) *libcluster.Cluster {
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
		// add command and turn on tracing for the rate limit exceeded check
		if len(logLevel) > 0 {
			cfg.Cmd = append(cfg.Cmd, fmt.Sprintf("-hcl=log_level=\"%s\"", logLevel))
		}
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
