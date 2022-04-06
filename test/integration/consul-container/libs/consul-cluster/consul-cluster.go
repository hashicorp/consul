package consul_cluster

import (
	"context"
	"fmt"
	"strings"

	consulnode "github.com/hashicorp/consul/integration/consul-container/libs/consul-node"
)

type Cluster struct {
	Nodes []consulnode.Node
}

func New(configs []consulnode.Config) (*Cluster, error) {
	cluster := Cluster{}

	for _, c := range configs {
		n, err := consulnode.NewConsulContainer(context.Background(), c)
		if err != nil {
			return nil, err
		}
		cluster.Nodes = append(cluster.Nodes, n)
	}
	err := cluster.join()
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (c *Cluster) join() error {
	if len(c.Nodes) < 2 {
		return nil
	}
	n0 := c.Nodes[0]
	for _, n := range c.Nodes {
		if n != n0 {
			addr, _ := n0.GetAddr()
			err := n.GetClient().Agent().Join(addr, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Cluster) AddNodes(nodes []consulnode.Node) error {
	n0 := c.Nodes[0]
	for _, node := range nodes {
		addr, _ := n0.GetAddr()
		err := node.GetClient().Agent().Join(addr, false)
		if err != nil {
			return err
		}
		c.Nodes = append(c.Nodes, node)
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

func (c *Cluster) Leader() (consulnode.Node, error) {
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
