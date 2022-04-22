package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/integration/consul-container/libs/node"
)

// Cluster provides an interface for creating and controlling a Consul cluster
// in integration tests, with nodes running in containers.
type Cluster struct {
	Nodes []node.Node
}

// New creates a Consul cluster. A node will be started for each of the given
// configs and joined to the cluster.
func New(configs []node.Config) (*Cluster, error) {
	cluster := Cluster{}

	nodes := make([]node.Node, len(configs))
	for idx, c := range configs {
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

	for _, node := range nodes {
		err := node.GetClient().Agent().Join(joinAddr, false)
		if err != nil {
			return err
		}
		c.Nodes = append(c.Nodes, node)
	}
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
	leaderAdd, err := n0.GetClient().Status().Leader()
	if err != nil {
		return nil, err
	}
	if leaderAdd == "" {
		return nil, fmt.Errorf("no leader available")
	}
	for _, n := range c.Nodes {
		addr, _ := n.GetAddr()
		if strings.Contains(leaderAdd, addr) {
			return n, nil
		}
	}
	return nil, fmt.Errorf("leader not found")
}
