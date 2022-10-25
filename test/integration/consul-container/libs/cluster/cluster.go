package cluster

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
)

// Cluster provides an interface for creating and controlling a Consul cluster
// in integration tests, with agents running in containers.
type Cluster struct {
	Agents     []agent.Agent
	EncryptKey string
}

// New creates a Consul cluster. A agent will be started for each of the given
// configs and joined to the cluster.
func New(configs []agent.Config) (*Cluster, error) {
	serfKey, err := newSerfEncryptionKey()
	if err != nil {
		return nil, err
	}

	cluster := Cluster{
		EncryptKey: serfKey,
	}

	agents := make([]agent.Agent, len(configs))
	for idx, c := range configs {
		// TODO (dans): replace with autoconfig
		//c.JSON += fmt.Sprintf("\nencrypt = %q", serfKey)

		n, err := agent.NewConsulContainer(context.Background(), c)
		if err != nil {
			return nil, err
		}
		agents[idx] = n
	}
	if err := cluster.Add(agents); err != nil {
		return nil, err
	}
	return &cluster, nil
}

// Add joins the given agent to the cluster.
func (c *Cluster) Add(agents []agent.Agent) error {
	var joinAddr string
	if len(c.Agents) >= 1 {
		joinAddr, _ = c.Agents[0].GetAddr()
	} else if len(agents) >= 1 {
		joinAddr, _ = agents[0].GetAddr()
	}

	for _, n := range agents {
		err := n.GetClient().Agent().Join(joinAddr, false)
		if err != nil {
			return err
		}
		c.Agents = append(c.Agents, n)
	}
	return nil
}

// Remove instructs the agent to leave the cluster then removes it
// from the cluster Agent list.
func (c *Cluster) Remove(n agent.Agent) error {
	err := n.GetClient().Agent().Leave()
	if err != nil {
		return err
	}

	foundIdx := -1
	for idx, this := range c.Agents {
		if this == n {
			foundIdx = idx
			break
		}
	}

	if foundIdx == -1 {
		return fmt.Errorf("could not find agent in cluster")
	}

	c.Agents = append(c.Agents[:foundIdx], c.Agents[foundIdx+1:]...)
	return nil
}

// Terminate will attempt to terminate all agents in the cluster. If any agent
// termination fails, Terminate will abort and return an error.
func (c *Cluster) Terminate() error {
	for _, n := range c.Agents {
		err := n.Terminate()
		if err != nil {
			return err
		}
	}
	return nil
}

// Leader returns the cluster leader agent, or an error if no leader is
// available.
func (c *Cluster) Leader() (agent.Agent, error) {
	if len(c.Agents) < 1 {
		return nil, fmt.Errorf("no agent available")
	}
	n0 := c.Agents[0]

	leaderAdd, err := GetLeader(n0.GetClient())
	if err != nil {
		return nil, err
	}

	for _, n := range c.Agents {
		addr, _ := n.GetAddr()
		if strings.Contains(leaderAdd, addr) {
			return n, nil
		}
	}
	return nil, fmt.Errorf("leader not found")
}

// Followers returns the cluster following servers.
func (c *Cluster) Followers() ([]agent.Agent, error) {
	var followers []agent.Agent

	leader, err := c.Leader()
	if err != nil {
		return nil, fmt.Errorf("could not determine leader: %w", err)
	}

	for _, n := range c.Agents {
		if n != leader && n.IsServer() {
			followers = append(followers, n)
		}
	}
	return followers, nil
}

// Servers returns the handle to server agents
func (c *Cluster) Servers() ([]agent.Agent, error) {
	var servers []agent.Agent

	for _, n := range c.Agents {
		if n.IsServer() {
			servers = append(servers, n)
		}
	}
	return servers, nil
}

// Clients returns the handle to client agents
func (c *Cluster) Clients() ([]agent.Agent, error) {
	var clients []agent.Agent

	for _, n := range c.Agents {
		if !n.IsServer() {
			clients = append(clients, n)
		}
	}
	return clients, nil
}

func newSerfEncryptionKey() (string, error) {
	key := make([]byte, 32)
	n, err := rand.Reader.Read(key)
	if err != nil {
		return "", fmt.Errorf("Error reading random data: %w", err)
	}
	if n != 32 {
		return "", fmt.Errorf("Couldn't read enough entropy. Generate more entropy!")
	}

	return base64.StdEncoding.EncodeToString(key), nil
}

func GetLeader(client *api.Client) (string, error) {
	leaderAdd, err := client.Status().Leader()
	if err != nil {
		return "", err
	}
	if leaderAdd == "" {
		return "", fmt.Errorf("no leader available")
	}
	return leaderAdd, nil
}

const retryTimeout = 20 * time.Second
const retryFrequency = 500 * time.Millisecond

func LongFailer() *retry.Timer {
	return &retry.Timer{Timeout: retryTimeout, Wait: retryFrequency}
}

func WaitForLeader(t *testing.T, cluster *Cluster, client *api.Client) {
	retry.RunWith(LongFailer(), t, func(r *retry.R) {
		leader, err := cluster.Leader()
		require.NoError(r, err)
		require.NotEmpty(r, leader)
	})

	if client != nil {
		waitForLeaderFromClient(t, client)
	}
}

func waitForLeaderFromClient(t *testing.T, client *api.Client) {
	retry.RunWith(LongFailer(), t, func(r *retry.R) {
		leader, err := GetLeader(client)
		require.NoError(r, err)
		require.NotEmpty(r, leader)
	})
}

func WaitForMembers(t *testing.T, client *api.Client, expectN int) {
	retry.RunWith(LongFailer(), t, func(r *retry.R) {
		members, err := client.Agent().Members(false)
		var activeMembers int
		for _, member := range members {
			if serf.MemberStatus(member.Status) == serf.StatusAlive {
				activeMembers++
			}
		}
		require.NoError(r, err)
		require.Equal(r, activeMembers, expectN)
	})
}
