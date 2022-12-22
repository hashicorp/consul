package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/test/integration/consul-container/test/topology"
)

// Given a 3-server cluster, when the leader is elected, then leader's isLeader is 1 and non-leader's 0
func TestLeadershipMetrics(t *testing.T) {
	cluster := topology.BasicSingleClusterTopology(t, &libcluster.Options{
		Datacenter: "dc1",
		NumServer:  3,
		NumClient:  1,
	})
	defer func() {
		err := cluster.Terminate()
		require.NoErrorf(t, err, "termining cluster")
	}()

	svrCli := cluster.Agents[0].GetClient()
	libcluster.WaitForLeader(t, cluster, svrCli)
	libcluster.WaitForMembers(t, svrCli, 4)

	retryWithBackoff := func(agent libcluster.Agent, expectedStr string) error {
		waiter := &libutils.Waiter{
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

	servers, err := cluster.Servers()
	require.NoError(t, err)
	for i, n := range servers {
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "nil", fmt.Errorf("error read metrics: %v", err)
	}
	return string(body), nil
}
