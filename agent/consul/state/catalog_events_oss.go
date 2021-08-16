// +build !consulent

package state

import "github.com/hashicorp/consul/agent/structs"

func (nst nodeServiceTuple) nodeTuple() nodeTuple {
	return nodeTuple{Node: nst.Node, Partition: ""}
}

func newNodeTupleFromNode(node *structs.Node) nodeTuple {
	return nodeTuple{
		Node:      node.Node,
		Partition: "",
	}
}

func newNodeTupleFromHealthCheck(hc *structs.HealthCheck) nodeTuple {
	return nodeTuple{
		Node:      hc.Node,
		Partition: "",
	}
}
