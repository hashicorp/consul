package state

import (
	"fmt"
	"os"
	"reflect"
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

func TestStateStore_GetNodes(t *testing.T) {
	s := testStateStore(t)

	// Create some nodes in the state store
	nodes := []*structs.Node{
		&structs.Node{Node: "node0", Address: "1.1.1.0"},
		&structs.Node{Node: "node1", Address: "1.1.1.1"},
		&structs.Node{Node: "node2", Address: "1.1.1.2"},
	}
	for i, node := range nodes {
		if err := s.EnsureNode(uint64(i), node); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Retrieve the nodes
	out, err := s.Nodes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// All nodes were returned
	if n := len(out); n != 3 {
		t.Fatalf("bad node count: %d", n)
	}

	// Make sure the nodes match
	for i, node := range nodes {
		if node.CreateIndex != uint64(i) || node.ModifyIndex != uint64(i) {
			t.Fatalf("bad node index: %d, %d", node.CreateIndex, node.ModifyIndex)
		}
		name := fmt.Sprintf("node%d", i)
		addr := fmt.Sprintf("1.1.1.%d", i)
		if node.Node != name || node.Address != addr {
			t.Fatalf("bad: %#v", node)
		}
	}
}

func TestStateStore_DeleteNode(t *testing.T) {
	s := testStateStore(t)

	// Create a node
	node := &structs.Node{Node: "node1", Address: "1.1.1.1"}
	if err := s.EnsureNode(1, node); err != nil {
		t.Fatalf("err: %s", err)
	}

	// The node exists
	if n, err := s.GetNode("node1"); err != nil || n == nil {
		t.Fatalf("bad: %#v (%#v)", n, err)
	}

	// Delete the node
	if err := s.DeleteNode(2, "node1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// The node is now gone and the index was updated
	if n, err := s.GetNode("node1"); err != nil || n != nil {
		t.Fatalf("bad: %#v (err: %#v)", node, err)
	}
	if idx := s.maxIndex("nodes"); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_EnsureService_NodeServices(t *testing.T) {
	s := testStateStore(t)

	// Fetching services for a node with none returns nil
	if res, err := s.NodeServices("node1"); err != nil || res != nil {
		t.Fatalf("expected (nil, nil), got: (%#v, %#v)", res, err)
	}

	// Register the nodes
	for i, nr := range []*structs.Node{
		&structs.Node{Node: "node1", Address: "1.1.1.1"},
		&structs.Node{Node: "node2", Address: "1.1.1.2"},
	} {
		if err := s.EnsureNode(uint64(i), nr); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Create the service registration
	ns1 := &structs.NodeService{
		ID:      "service1",
		Service: "redis",
		Tags:    []string{"prod"},
		Address: "1.1.1.1",
		Port:    1111,
	}

	// Service successfully registers into the state store
	if err := s.EnsureService(10, "node1", ns1); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Register a similar service against both nodes
	ns2 := *ns1
	ns2.ID = "service2"
	for _, n := range []string{"node1", "node2"} {
		if err := s.EnsureService(20, n, &ns2); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Register a different service on the bad node
	ns3 := *ns1
	ns3.ID = "service3"
	if err := s.EnsureService(30, "node2", &ns3); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the services
	out, err := s.NodeServices("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Only the services for the requested node are returned
	if out == nil || len(out.Services) != 2 {
		t.Fatalf("bad services: %#v", out)
	}

	// Results match the inserted services and have the proper indexes set
	expect1 := *ns1
	expect1.CreateIndex, expect1.ModifyIndex = 10, 10
	if svc := out.Services["service1"]; !reflect.DeepEqual(&expect1, svc) {
		t.Fatalf("bad: %#v", svc)
	}

	expect2 := ns2
	expect2.CreateIndex, expect2.ModifyIndex = 20, 20
	if svc := out.Services["service2"]; !reflect.DeepEqual(&expect2, svc) {
		t.Fatalf("bad: %#v %#v", ns2, svc)
	}

	// Lastly, ensure that the highest index was preserved.
	if out.CreateIndex != 20 || out.ModifyIndex != 20 {
		t.Fatalf("bad index: %d, %d", out.CreateIndex, out.ModifyIndex)
	}
}

func TestStateStore_DeleteNodeService(t *testing.T) {
	s := testStateStore(t)

	// Register a node
	node := &structs.Node{
		Node:    "node1",
		Address: "1.1.1.1",
	}
	if err := s.EnsureNode(1, node); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Create a service
	service := &structs.NodeService{
		ID:      "service1",
		Service: "redis",
		Tags:    []string{"prod"},
		Address: "1.1.1.1",
		Port:    1111,
	}
	if err := s.EnsureService(2, "node1", service); err != nil {
		t.Fatalf("err: %s", err)
	}

	// The service exists
	ns, err := s.NodeServices("node1")
	if err != nil || ns == nil || len(ns.Services) != 1 {
		t.Fatalf("bad: %#v (err: %#v)", ns, err)
	}

	// Delete the service
	if err := s.DeleteNodeService(3, "node1", "service1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// The service doesn't exist and the index was updated
	ns, err = s.NodeServices("node1")
	if err != nil || ns == nil || len(ns.Services) != 0 {
		t.Fatalf("bad: %#v (err: %#v)", ns, err)
	}
	if idx := s.maxIndex("services"); idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_EnsureCheck(t *testing.T) {
	s := testStateStore(t)

	// Create a node and insert it
	node := &structs.Node{
		Node:    "node1",
		Address: "1.1.1.1",
	}
	if err := s.EnsureNode(1, node); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Create a service and insert it
	service := &structs.NodeService{
		ID:      "service1",
		Service: "redis",
		Tags:    []string{"prod"},
		Address: "1.1.1.1",
		Port:    1111,
	}
	if err := s.EnsureService(2, "node1", service); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Create a check associated with the node and insert it
	check := &structs.HealthCheck{
		Node:        "node1",
		CheckID:     "check1",
		Name:        "redis check",
		Status:      structs.HealthPassing,
		Notes:       "test check",
		Output:      "aaa",
		ServiceID:   "service1",
		ServiceName: "redis",
	}
	if err := s.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the check and make sure it matches
	checks, err := s.NodeChecks("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 1 {
		t.Fatalf("bad number of checks: %d", len(checks))
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %#v", checks[0])
	}
}
