package consul_cluster

import (
	"context"
	"fmt"
	"strings"

	consulcontainer "github.com/hashicorp/consul/integration/ca/libs/consul-node"
)

type Cluster struct {
	Nodes []consulcontainer.ConsulNode
}

func New(configs []consulcontainer.Config) (*Cluster, error) {
	cluster := Cluster{}

	for _, c := range configs {
		n, err := consulcontainer.NewNodeWitConfig(context.Background(), c)
		if err != nil {
			return nil, err
		}
		cluster.Nodes = append(cluster.Nodes, *n)
	}
	err := cluster.join()
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (c *Cluster) join() error {
	if len(c.Nodes) < 2 {
		return fmt.Errorf("cluster have only %d nodes", len(c.Nodes))
	}
	n0 := c.Nodes[0]
	for _, n := range c.Nodes {
		if n != n0 {
			err := n.Client.Agent().Join(n0.IP, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Cluster) AddNodes(nodes []*consulcontainer.ConsulNode) error {
	n0 := c.Nodes[0]
	for _, node := range nodes {
		err := node.Client.Agent().Join(n0.IP, false)
		if err != nil {
			return err
		}
		c.Nodes = append(c.Nodes, *node)
	}
	return nil
}

func (c *Cluster) Terminate() error {
	for _, n := range c.Nodes {
		err := n.Terminate()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) Leader() (*consulcontainer.ConsulNode, error) {
	if len(c.Nodes) < 1 {
		return nil, fmt.Errorf("no node available")
	}
	n0 := c.Nodes[0]
	leaderAdd, err := n0.Client.Status().Leader()
	if err != nil {
		return nil, err
	}
	if leaderAdd == "" {
		return nil, fmt.Errorf("no leader available")
	}
	for _, n := range c.Nodes {
		if strings.Contains(leaderAdd, n.IP) {
			return &n, nil
		}
	}
	return nil, fmt.Errorf("leader not found")
}
