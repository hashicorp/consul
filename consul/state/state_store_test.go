package state

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

func testStateStore(t *testing.T) *StateStore {
	s, err := NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if s == nil {
		t.Fatalf("missing state store")
	}
	return s
}

func TestStateStore_EnsureNode(t *testing.T) {
	s := testStateStore(t)

	// Create a node registration request
	in := &structs.Node{
		Node:    "node1",
		Address: "1.1.1.1",
	}

	// Ensure the node is registered in the db
	if err := s.EnsureNode(1, in); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the node again
	out, err := s.GetNode("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Correct node was returned
	if out.Node != "node1" || out.Address != "1.1.1.1" {
		t.Fatalf("bad node returned: %#v", out)
	}

	// Indexes are set properly
	if out.CreateIndex != 1 || out.ModifyIndex != 1 {
		t.Fatalf("bad node index: %#v", out)
	}

	// Update the node registration
	in.Address = "1.1.1.2"
	if err := s.EnsureNode(2, in); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the node
	out, err = s.GetNode("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Node and indexes were updated
	if out.CreateIndex != 1 || out.ModifyIndex != 2 || out.Address != "1.1.1.2" {
		t.Fatalf("bad: %#v", out)
	}
}
