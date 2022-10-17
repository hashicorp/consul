package cluster

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/node"
)

// Cluster provides an interface for creating and controlling a Consul cluster
// in integration tests, with nodes running in containers.
type Cluster struct {
	Nodes      []node.Node
	EncryptKey string
}

// New creates a Consul cluster. A node will be started for each of the given
// configs and joined to the cluster.
func New(configs []node.Config) (*Cluster, error) {
	serfKey, err := newSerfEncryptionKey()
	if err != nil {
		return nil, err
	}

	cluster := Cluster{
		EncryptKey: serfKey,
	}

	nodes := make([]node.Node, len(configs))
	for idx, c := range configs {
		c.HCL += fmt.Sprintf(" encrypt=%q", serfKey)

		n, err := node.NewConsulContainer(context.Background(), c)
		if err != nil {
			return nil, err
		}
		nodes[idx] = n
	}
	if err := cluster.AddNodes(nodes); err != nil {
		return nil, err
	}
	return &cluster, nil
}

// AddNodes joins the given nodes to the cluster.
func (c *Cluster) AddNodes(nodes []node.Node) error {
	var joinAddr string
	if len(c.Nodes) >= 1 {
		joinAddr, _ = c.Nodes[0].GetAddr()
	} else if len(nodes) >= 1 {
		joinAddr, _ = nodes[0].GetAddr()
	}

	for _, n := range nodes {
		err := n.GetClient().Agent().Join(joinAddr, false)
		if err != nil {
			return err
		}
		c.Nodes = append(c.Nodes, n)
	}
	return nil
}

// RemoveNode instructs the node to leave the cluster then removes it
// from the cluster Node list.
func (c *Cluster) RemoveNode(n node.Node) error {
	err := n.GetClient().Agent().Leave()
	if err != nil {
		return err
	}

	foundIdx := -1
	for idx, this := range c.Nodes {
		if this == n {
			foundIdx = idx
			break
		}
	}

	if foundIdx == -1 {
		return fmt.Errorf("could not find node in cluster")
	}

	c.Nodes = append(c.Nodes[:foundIdx], c.Nodes[foundIdx+1:]...)
	return nil
}

// Terminate will attempt to terminate all nodes in the cluster. If any node
// termination fails, Terminate will abort and return an error.
func (c *Cluster) Terminate() error {
	for _, n := range c.Nodes {
		err := n.Terminate()
		if err != nil {
			return err
		}
	}
	return nil
}

// Leader returns the cluster leader node, or an error if no leader is
// available.
func (c *Cluster) Leader() (node.Node, error) {
	if len(c.Nodes) < 1 {
		return nil, fmt.Errorf("no node available")
	}
	n0 := c.Nodes[0]

	leaderAdd, err := GetLeader(n0.GetClient())
	if err != nil {
		return nil, err
	}

	for _, n := range c.Nodes {
		addr, _ := n.GetAddr()
		if strings.Contains(leaderAdd, addr) {
			return n, nil
		}
	}
	return nil, fmt.Errorf("leader not found")
}

// Followers returns the cluster following servers.
func (c *Cluster) Followers() ([]node.Node, error) {
	var followers []node.Node

	leader, err := c.Leader()
	if err != nil {
		return nil, fmt.Errorf("could not determine leader: %w", err)
	}

	for _, n := range c.Nodes {
		info, err := n.GetClient().Agent().Self()
		consulBuf := info["Stats"]["consul"].(map[string]interface{})
		isServer, err := strconv.ParseBool(consulBuf["server"].(string))
		if err != nil {
			return nil, fmt.Errorf("could not parse agent self response: %w", err)
		}

		if n != leader && isServer {
			followers = append(followers, n)
		}
	}
	return followers, nil
}

// Clients returns the handle to client agent nodes.
func (c *Cluster) Clients() ([]node.Node, error) {
	var clients []node.Node

	for _, n := range c.Nodes {
		info, err := n.GetClient().Agent().Self()
		consulBuf := info["Stats"]["consul"].(map[string]interface{})
		isServer, err := strconv.ParseBool(consulBuf["server"].(string))
		if err != nil {
			return nil, fmt.Errorf("could not parse agent self response: %w", err)
		}

		if !isServer {
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
