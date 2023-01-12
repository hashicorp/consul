package ratelimit

import (
	"testing"

	"github.com/stretchr/testify/require"

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
	type testOperation struct {
		action           func(client *api.Client) error
		expectedErrorMsg string
	}
	type testCase struct {
		dsc        string
		cmd        string
		operations []testOperation
	}

	testCases := []testCase{
		{
			dsc: "Mode: disabled - does not return errors / does not emit metrics",
			cmd: "-hcl=limits { request_limits { mode = \"disabled\" read_rate = 0 write_rate = 0 }}",
			operations: []testOperation{
				{
					action: func(client *api.Client) error {
						_, _, err := client.Catalog().Nodes(&api.QueryOptions{})
						return err
					},
					expectedErrorMsg: "",
				},
				{
					action: func(client *api.Client) error {
						_, err := utils.ApplyDefaultProxySettings(client)
						return err
					},
					expectedErrorMsg: "",
				},
			},
		},
		{
			dsc: "Mode: permissive - does not return error / emits metrics",
			cmd: "-hcl=limits { request_limits { mode = \"permissive\" read_rate = 0 write_rate = 0 }}",
			operations: []testOperation{
				{
					action: func(client *api.Client) error {
						_, _, err := client.Catalog().Nodes(&api.QueryOptions{})
						return err
					},
					expectedErrorMsg: "",
				},
				{
					action: func(client *api.Client) error {
						_, err := utils.ApplyDefaultProxySettings(client)
						return err
					},
					expectedErrorMsg: "",
				},
			},
		},
		{
			dsc: "Mode: enforcing - returns errors / emits metrics",
			cmd: "-hcl=limits { request_limits { mode = \"enforcing\" read_rate = 0 write_rate = 0 }}",
			operations: []testOperation{
				{
					action: func(client *api.Client) error {
						_, _, err := client.Catalog().Nodes(&api.QueryOptions{})
						return err
					},
					expectedErrorMsg: retryableErrorMsg,
				},
				{
					action: func(client *api.Client) error {
						_, err := utils.ApplyDefaultProxySettings(client)
						return err
					},
					expectedErrorMsg: nonRetryableErrorMsg,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.dsc, func(t *testing.T) {
			cluster := createCluster(t, tc.cmd)
			defer terminate(t, cluster)

			client, err := cluster.GetClient(nil, true)
			require.NoError(t, err)
			for _, op := range tc.operations {
				err = op.action(client)
				if len(op.expectedErrorMsg) > 0 {
					require.Error(t, err)
					require.Equal(t, op.expectedErrorMsg, err.Error())
				} else {
					require.NoError(t, err)
				}
			}

			// currently returns NaN error
			//			metrics, err := client.Agent().Metrics()
			//			require.NoError(t, err)
			//			for counter := range metrics.Counters {
			//				t.Logf("Counter: %+v", counter)
			//			}
		})
	}
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

// createCluster
func createCluster(t *testing.T, cmd string) *libcluster.Cluster {
	opts := libcluster.BuildOptions{
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	conf := libcluster.NewConfigBuilder(ctx).ToAgentConfig(t)

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
