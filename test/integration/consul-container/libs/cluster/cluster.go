package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"github.com/teris-io/shortid"
	"github.com/testcontainers/testcontainers-go"
)

// Cluster provides an interface for creating and controlling a Consul cluster
// in integration tests, with agents running in containers.
// These fields are public in the event someone might want to surgically
// craft a test case.
type Cluster struct {
	Agents []Agent
	// BuildContext *BuildContext // TODO
	CACert      string
	CAKey       string
	ID          string
	Index       int
	Network     testcontainers.Network
	NetworkName string
	ScratchDir  string
}

type TestingT interface {
	Logf(format string, args ...any)
	Cleanup(f func())
}

func NewN(t TestingT, conf Config, count int) (*Cluster, error) {
	var configs []Config
	for i := 0; i < count; i++ {
		configs = append(configs, conf)
	}

	return New(t, configs)
}

// New creates a Consul cluster. An agent will be started for each of the given
// configs and joined to the cluster.
//
// A cluster has its own docker network for DNS connectivity, but is also
// joined
//
// The provided TestingT is used to register a cleanup function to terminate
// the cluster.
func New(t TestingT, configs []Config) (*Cluster, error) {
	id, err := shortid.Generate()
	if err != nil {
		return nil, fmt.Errorf("could not cluster id: %w", err)
	}

	name := fmt.Sprintf("consul-int-cluster-%s", id)
	network, err := createNetwork(t, name)
	if err != nil {
		return nil, fmt.Errorf("could not create cluster container network: %w", err)
	}

	// Rig up one scratch dir for the cluster with auto-cleanup on test exit.
	scratchDir, err := os.MkdirTemp("", name)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(scratchDir)
	})
	err = os.Chmod(scratchDir, 0777)
	if err != nil {
		return nil, err
	}

	cluster := &Cluster{
		ID:          id,
		Network:     network,
		NetworkName: name,
		ScratchDir:  scratchDir,
	}
	t.Cleanup(func() {
		_ = cluster.Terminate()
	})

	if err := cluster.Add(configs, true); err != nil {
		return nil, fmt.Errorf("could not start or join all agents: %w", err)
	}

	return cluster, nil
}

func (c *Cluster) AddN(conf Config, count int, join bool) error {
	var configs []Config
	for i := 0; i < count; i++ {
		configs = append(configs, conf)
	}
	return c.Add(configs, join)
}

// Add starts an agent with the given configuration and joins it with the existing cluster
func (c *Cluster) Add(configs []Config, serfJoin bool) (xe error) {
	if c.Index == 0 && !serfJoin {
		return fmt.Errorf("The first call to Cluster.Add must have serfJoin=true")
	}

	var agents []Agent
	for idx, conf := range configs {
		// Each agent gets it's own area in the cluster scratch.
		conf.ScratchDir = filepath.Join(c.ScratchDir, strconv.Itoa(c.Index))
		if err := os.MkdirAll(conf.ScratchDir, 0777); err != nil {
			return err
		}
		if err := os.Chmod(conf.ScratchDir, 0777); err != nil {
			return err
		}

		n, err := NewConsulContainer(
			context.Background(),
			conf,
			c.NetworkName,
			c.Index,
		)
		if err != nil {
			return fmt.Errorf("could not add container index %d: %w", idx, err)
		}
		agents = append(agents, n)
		c.Index++
	}

	if serfJoin {
		if err := c.Join(agents); err != nil {
			return fmt.Errorf("could not join agents to cluster: %w", err)
		}
	} else {
		if err := c.JoinExternally(agents); err != nil {
			return fmt.Errorf("could not join agents to cluster: %w", err)
		}
	}

	return nil
}

// Join joins the given agent to the cluster.
func (c *Cluster) Join(agents []Agent) error {
	return c.join(agents, false)
}
func (c *Cluster) JoinExternally(agents []Agent) error {
	return c.join(agents, true)
}
func (c *Cluster) join(agents []Agent, skipSerfJoin bool) error {
	if len(agents) == 0 {
		return nil // no change
	}

	if len(c.Agents) == 0 {
		// Join the rest to the first.
		c.Agents = append(c.Agents, agents[0])
		return c.join(agents[1:], skipSerfJoin)
	}

	// Always join to the original server.
	joinAddr := c.Agents[0].GetIP()

	for _, n := range agents {
		if !skipSerfJoin {
			err := n.GetClient().Agent().Join(joinAddr, false)
			if err != nil {
				return fmt.Errorf("could not join agent %s to %s: %w", n.GetName(), joinAddr, err)
			}
		}
		c.Agents = append(c.Agents, n)
	}
	return nil
}

// Remove instructs the agent to leave the cluster then removes it
// from the cluster Agent list.
func (c *Cluster) Remove(n Agent) error {
	err := n.GetClient().Agent().Leave()
	if err != nil {
		return fmt.Errorf("could not remove agent %s: %w", n.GetName(), err)
	}

	foundIdx := -1
	for idx, this := range c.Agents {
		if this == n {
			foundIdx = idx
			break
		}
	}

	if foundIdx == -1 {
		return errors.New("could not find agent in cluster")
	}

	c.Agents = append(c.Agents[:foundIdx], c.Agents[foundIdx+1:]...)
	return nil
}

// StandardUpgrade upgrades a running consul cluster following the steps from
//
//	https://developer.hashicorp.com/consul/docs/upgrading#standard-upgrades
//
// - takes a snapshot (which is discarded)
// - terminate and rejoin the pod of a new version of consul
//
// NOTE: we pass in a *testing.T but this method also returns an error. JUST
// within this method when in doubt return an error. A testing assertion should
// be saved only for using t.Cleanup or in a few of the retry-until-working
// helpers below.
//
// This lets us have tests that assert that an upgrade will fail.
func (c *Cluster) StandardUpgrade(t *testing.T, ctx context.Context, targetVersion string) error {
	// We take a snapshot, but note that we currently do nothing with it.
	execCode, err := c.Agents[0].Exec(context.Background(), []string{"consul", "snapshot", "save", "backup.snap"})
	if execCode != 0 {
		return fmt.Errorf("error taking snapshot of the cluster, returned code %d", execCode)
	}
	if err != nil {
		return err
	}

	// Upgrade individual agent to the target version in the following order
	// 1. followers
	// 2. leader
	// 3. clients (TODO)

	// Grab a client connected to the leader, which we will upgrade last so our
	// connection remains ok.
	leader, err := c.Leader()
	if err != nil {
		return err
	}
	t.Logf("Leader name: %s", leader.GetName())

	followers, err := c.Followers()
	if err != nil {
		return err
	}
	t.Logf("The number of followers = %d", len(followers))

	upgradeFn := func(agent Agent, clientFactory func() *api.Client) error {
		config := agent.GetConfig()
		config.Version = targetVersion

		if agent.IsServer() {
			// You only ever need bootstrap settings the FIRST time, so we do not need
			// them again.
			config.ConfigBuilder.Unset("bootstrap")
		} else {
			// If we upgrade the clients fast enough
			// membership might not be gossiped to all of
			// the clients to persist into their serf
			// snapshot, so force them to rejoin the
			// normal way on restart.
			config.ConfigBuilder.Set("retry_join", []string{"agent-0"})
		}

		newJSON, err := json.MarshalIndent(config.ConfigBuilder, "", "  ")
		if err != nil {
			return fmt.Errorf("could not re-generate json config: %w", err)
		}
		config.JSON = string(newJSON)
		t.Logf("Upgraded cluster config for %q:\n%s", agent.GetName(), config.JSON)

		err = agent.Upgrade(context.Background(), config)
		if err != nil {
			return err
		}

		client := clientFactory()

		// wait until the agent rejoin
		WaitForMembers(t, client, len(c.Agents))

		return nil
	}

	for _, agent := range followers {
		t.Logf("Upgrade follower: %s", agent.GetName())

		if err := upgradeFn(agent, leader.GetClient); err != nil {
			return fmt.Errorf("error upgrading follower %q: %w", agent.GetName(), err)
		}
	}

	t.Logf("Upgrade leader: %s", leader.GetName())
	err = upgradeFn(leader, func() *api.Client {
		if len(followers) > 0 {
			return followers[0].GetClient()
		}
		return c.APIClient(0)
	})
	if err != nil {
		return fmt.Errorf("error upgrading leader %q: %w", leader.GetName(), err)
	}

	return nil
}

// Terminate will attempt to terminate all agents in the cluster and its network. If any agent
// termination fails, Terminate will abort and return an error.
func (c *Cluster) Terminate() error {
	for _, n := range c.Agents {
		err := n.Terminate()
		if err != nil {
			return err
		}
	}

	// Testcontainers seems to clean this the network.
	// Trigger it now will throw an error while the containers are still shutting down
	// if err := c.Network.Remove(context.Background()); err != nil {
	// 	return fmt.Errorf("could not terminate cluster network %s: %w", c.ID, err)
	// }
	return nil
}

// Leader returns the cluster leader agent, or an error if no leader is
// available.
func (c *Cluster) Leader() (Agent, error) {
	if len(c.Agents) < 1 {
		return nil, fmt.Errorf("no agent available")
	}
	n0 := c.Agents[0]

	leaderAdd, err := getLeader(n0.GetClient())
	if err != nil {
		return nil, err
	}

	for _, n := range c.Agents {
		addr := n.GetIP()
		if strings.Contains(leaderAdd, addr) {
			return n, nil
		}
	}
	return nil, fmt.Errorf("leader not found")
}

func getLeader(client *api.Client) (string, error) {
	leaderAdd, err := client.Status().Leader()
	if err != nil {
		return "", fmt.Errorf("could not query leader: %w", err)
	}
	if leaderAdd == "" {
		return "", errors.New("no leader available")
	}
	return leaderAdd, nil
}

// Followers returns the cluster following servers.
func (c *Cluster) Followers() ([]Agent, error) {
	var followers []Agent

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
func (c *Cluster) Servers() []Agent {
	var servers []Agent

	for _, n := range c.Agents {
		if n.IsServer() {
			servers = append(servers, n)
		}
	}
	return servers
}

// Clients returns the handle to client agents
func (c *Cluster) Clients() []Agent {
	var clients []Agent

	for _, n := range c.Agents {
		if !n.IsServer() {
			clients = append(clients, n)
		}
	}
	return clients
}

func (c *Cluster) APIClient(index int) *api.Client {
	nodes := c.Clients()
	if len(nodes) == 0 {
		nodes = c.Servers()
		if len(nodes) == 0 {
			return nil
		}
	}
	return nodes[0].GetClient()
}

// GetClient returns a consul API client to the node if node is provided.
// Otherwise, GetClient returns the API client to the first node of either
// server or client agent.
//
// TODO: see about switching to just APIClient() calls instead?
func (c *Cluster) GetClient(node Agent, isServer bool) (*api.Client, error) {
	if node != nil {
		return node.GetClient(), nil
	}

	nodes := c.Clients()
	if isServer {
		nodes = c.Servers()
	}

	if len(nodes) <= 0 {
		return nil, errors.New("no nodes")
	}

	return nodes[0].GetClient(), nil
}

// PeerWithCluster establishes peering with the acceptor cluster
func (c *Cluster) PeerWithCluster(acceptingClient *api.Client, acceptingPeerName string, dialingPeerName string) error {
	dialingClient := c.APIClient(0)

	generateReq := api.PeeringGenerateTokenRequest{
		PeerName: acceptingPeerName,
	}
	generateRes, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), generateReq, &api.WriteOptions{})
	if err != nil {
		return fmt.Errorf("error generate token: %w", err)
	}

	establishReq := api.PeeringEstablishRequest{
		PeerName:     dialingPeerName,
		PeeringToken: generateRes.PeeringToken,
	}
	_, _, err = dialingClient.Peerings().Establish(context.Background(), establishReq, &api.WriteOptions{})
	if err != nil {
		return fmt.Errorf("error establish peering: %w", err)
	}

	return nil
}

const retryTimeout = 90 * time.Second
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
		leader, err := getLeader(client)
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
		require.Equal(r, expectN, activeMembers)
	})
}
