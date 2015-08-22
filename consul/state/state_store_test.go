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

func TestStateStore_EnsureNode_GetNode(t *testing.T) {
	s := testStateStore(t)

	// Fetching a non-existent node returns nil
	if node, err := s.GetNode("node1"); node != nil || err != nil {
		t.Fatalf("expected (nil, nil), got: (%#v, %#v)", node, err)
	}

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

	// Node upsert is idempotent
	if err := s.EnsureNode(2, in); err != nil {
		t.Fatalf("err: %s", err)
	}
	out, err = s.GetNode("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.Address != "1.1.1.2" || out.CreateIndex != 1 || out.ModifyIndex != 2 {
		t.Fatalf("node was modified: %#v", out)
	}
}

func TestStateStore_EnsureService(t *testing.T) {
	s := testStateStore(t)

	// Create the service registration
	in := &structs.NodeService{
		ID:      "service1",
		Service: "redis",
		Tags:    []string{"prod"},
		Address: "1.1.1.1",
		Port:    1111,
	}

	// Service successfully registers into the state store
	if err := s.EnsureService(1, in); err != nil {
		t.Fatalf("err: %s", err)
	}
}
