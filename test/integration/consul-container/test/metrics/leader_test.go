package metrics

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	libagent "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Given a 3-server cluster, when the leader is elected, then leader's isLeader is 1 and non-leader's 0
func TestLeadershipMetrics(t *testing.T) {
	var configs []agent.Config

	statsConf, err := libagent.NewConfigBuilder(nil).Telemetry("127.0.0.0:2180").ToAgentConfig()
	require.NoError(t, err)
	configs = append(configs, *statsConf)

	conf, err := libagent.NewConfigBuilder(nil).Bootstrap(3).ToAgentConfig()
	require.NoError(t, err)
	numServer := 3
	for i := 1; i < numServer; i++ {
		configs = append(configs, *conf)
	}

	cluster, err := libcluster.New(configs)
	require.NoError(t, err)
	defer terminate(t, cluster)

	svrCli := cluster.Agents[0].GetClient()
	libcluster.WaitForLeader(t, cluster, svrCli)
	libcluster.WaitForMembers(t, svrCli, 3)

	retryWithBackoff := func(agent agent.Agent, expectedStr string) error {
		waiter := &utils.Waiter{
			MaxWait: 5 * time.Minute,
		}
		_, port := agent.GetAddr()
		ctx := context.Background()
		for {
			if waiter.Failures() > 5 {
				return fmt.Errorf("reach max failure: %d", waiter.Failures())
			}

			metricsStr, err := getMetrics(t, "127.0.0.1", port, "/v1/agent/metrics")
			if err != nil {
				return fmt.Errorf("error get metrics: %v", err)
			}
			if strings.Contains(metricsStr, expectedStr) {
				return nil
			}
			waiter.Wait(ctx)
		}
	}

	leader, err := cluster.Leader()
	require.NoError(t, err)
	leadAddr, leaderPort := leader.GetAddr()

	for i, n := range cluster.Agents {
		addr, port := n.GetAddr()
		if addr == leadAddr && port == leaderPort {
			err = retryWithBackoff(leader, ".server.isLeader\",\"Value\":1,")
			require.NoError(t, err, "%dth node(leader): could not find the metric %q in the /v1/agent/metrics response", i, ".server.isLeader\",\"Value\":1,")
		} else {
			err = retryWithBackoff(n, ".server.isLeader\",\"Value\":0,")
			require.NoError(t, err, "%dth node(non-leader): could not find the metric %q in the /v1/agent/metrics response", i, ".server.isLeader\",\"Value\":0,")
		}
	}
}

func getMetrics(t *testing.T, addr string, port int, path string) (string, error) {
	u, err := url.Parse(fmt.Sprintf("http://%s:%d", addr, port))
	require.NoError(t, err)
	u.Path = path
	resp, err := http.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("error get metrics: %v", err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "nil", fmt.Errorf("error read metrics: %v", err)
	}
	return string(body), nil
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}
