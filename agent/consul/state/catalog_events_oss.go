//go:build !consulent
// +build !consulent

package state

import (
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func (nst nodeServiceTuple) nodeTuple() nodeTuple {
	return nodeTuple{
		Node:      strings.ToLower(nst.Node),
		Partition: "",
	}
}

func newNodeTupleFromNode(node *structs.Node) nodeTuple {
	return nodeTuple{
		Node:      strings.ToLower(node.Node),
		Partition: "",
	}
}

func newNodeTupleFromHealthCheck(hc *structs.HealthCheck) nodeTuple {
	return nodeTuple{
		Node:      strings.ToLower(hc.Node),
		Partition: "",
	}
}
