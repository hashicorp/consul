package main

import (
	"fmt"

	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul-topology/topology"
)

func makeServers(names []string, addrs []*topology.Address) []*topology.Node {
	var out []*topology.Node
	for _, name := range names {
		out = append(out, &topology.Node{
			Kind:      topology.NodeKindServer,
			Name:      name,
			Addresses: duplicate(addrs).([]*topology.Address),
		})
	}
	return out
}

func duplicate(v any) any {
	v2, err := copystructure.Copy(v)
	if err != nil {
		panic(fmt.Errorf("could not copy %T: %v", v, err))
	}
	return v2
}
