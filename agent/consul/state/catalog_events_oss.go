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
		PeerName:  nst.PeerName,
	}
}

func newNodeTupleFromNode(node *structs.Node) nodeTuple {
	return nodeTuple{
		Node:      strings.ToLower(node.Node),
		Partition: "",
		PeerName:  node.PeerName,
	}
}

func newNodeTupleFromHealthCheck(hc *structs.HealthCheck) nodeTuple {
	return nodeTuple{
		Node:      strings.ToLower(hc.Node),
		Partition: "",
		PeerName:  hc.PeerName,
	}
}

// String satisfies the stream.Subject interface.
func (s EventSubjectService) String() string {
	key := s.Key
	if v := s.overrideKey; v != "" {
		key = v
	}
	key = strings.ToLower(key)

	if s.PeerName == "" {
		return key
	}
	return s.PeerName + "/" + key
}
