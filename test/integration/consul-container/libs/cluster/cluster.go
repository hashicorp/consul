package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/integration/consul-container/libs/node"
)

// Cluster abstract a Consul Cluster by providing
// a way to create and join a Consul Cluster
// a way to add nodes to a cluster
// a way to fetch the cluster leader...
type Cluster struct {
	Nodes []node.Node
}

// New Create a new cluster based on the provided configuration
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

// AddNodes add a number of nodes to the current cluster and join them to the cluster
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

// Terminate will attempt to terminate all the nodes in the cluster
// if a node termination fail, Terminate will abort and return and error
func (c *Cluster) Terminate() error {
	for _, n := range c.Nodes {
		err := n.Terminate()
		if err != nil {
			return err
		}
	}
	return nil
}

// Leader return the cluster leader node
// if no leader is available or the leader is not part of the cluster
// an error will be returned
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
