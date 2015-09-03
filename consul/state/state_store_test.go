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

func testRegisterNode(t *testing.T, s *StateStore, idx uint64, nodeID string) {
	node := &structs.Node{Node: nodeID}
	if err := s.EnsureNode(idx, node); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	n, err := tx.First("nodes", "id", nodeID)
	if err != nil {
		t.Fatalf("err: %s", err, n)
	}
	if result, ok := n.(*structs.Node); !ok || result.Node != nodeID {
		t.Fatalf("bad node: %#v", result)
	}
}

func testRegisterService(t *testing.T, s *StateStore, idx uint64, nodeID, serviceID string) {
	svc := &structs.NodeService{
		ID:      serviceID,
		Service: serviceID,
		Address: "1.1.1.1",
		Port:    1111,
	}
	if err := s.EnsureService(idx, nodeID, svc); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	service, err := tx.First("services", "id", nodeID, serviceID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := service.(*structs.ServiceNode); !ok || result.ServiceID != serviceID {
		t.Fatalf("bad service: %#v", result)
	}
}

func testRegisterCheck(t *testing.T, s *StateStore, idx uint64,
	nodeID, serviceID, checkID, state string) {
	chk := &structs.HealthCheck{
		Node:      nodeID,
		CheckID:   checkID,
		ServiceID: serviceID,
		Status:    state,
	}
	if err := s.EnsureCheck(idx, chk); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	c, err := tx.First("checks", "id", nodeID, checkID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := c.(*structs.HealthCheck); !ok || result.CheckID != checkID {
		t.Fatalf("bad check: %#v", result)
	}
}

func testSetKey(t *testing.T, s *StateStore, idx uint64, key, value string) {
	entry := &structs.DirEntry{Key: key, Value: []byte(value)}
	if err := s.KVSSet(idx, entry); err != nil {
		t.Fatalf("err: %s", err)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()
	e, err := tx.First("kvs", "id", key)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result, ok := e.(*structs.DirEntry); !ok || result.Key != key {
		t.Fatalf("bad kvs entry: %#v", result)
	}
}

func TestStateStore_EnsureNode(t *testing.T) {
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

	// Index tables were updated
	if idx := s.maxIndex("nodes"); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_GetNodes(t *testing.T) {
	s := testStateStore(t)

	// Create some nodes in the state store
	testRegisterNode(t, s, 0, "node0")
	testRegisterNode(t, s, 1, "node1")
	testRegisterNode(t, s, 2, "node2")

	// Retrieve the nodes
	idx, nodes, err := s.Nodes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Highest index was returned
	if idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	// All nodes were returned
	if n := len(nodes); n != 3 {
		t.Fatalf("bad node count: %d", n)
	}

	// Make sure the nodes match
	for i, node := range nodes {
		if node.CreateIndex != uint64(i) || node.ModifyIndex != uint64(i) {
			t.Fatalf("bad node index: %d, %d", node.CreateIndex, node.ModifyIndex)
		}
		name := fmt.Sprintf("node%d", i)
		if node.Node != name {
			t.Fatalf("bad: %#v", node)
		}
	}
}

func TestStateStore_DeleteNode(t *testing.T) {
	s := testStateStore(t)

	// Create a node and register a service and health check with it.
	testRegisterNode(t, s, 0, "node1")
	testRegisterService(t, s, 1, "node1", "service1")
	testRegisterCheck(t, s, 2, "node1", "", "check1", structs.HealthPassing)

	// Delete the node
	if err := s.DeleteNode(3, "node1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// The node was removed
	if n, err := s.GetNode("node1"); err != nil || n != nil {
		t.Fatalf("bad: %#v (err: %#v)", n, err)
	}

	// Associated service was removed. Need to query this directly out of
	// the DB to make sure it is actually gone.
	tx := s.db.Txn(false)
	defer tx.Abort()
	services, err := tx.Get("services", "id", "node1", "service1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if s := services.Next(); s != nil {
		t.Fatalf("bad: %#v", s)
	}

	// Associated health check was removed.
	checks, err := tx.Get("checks", "id", "node1", "check1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if c := checks.Next(); c != nil {
		t.Fatalf("bad: %#v", c)
	}

	// Indexes were updated.
	for _, tbl := range []string{"nodes", "services", "checks"} {
		if idx := s.maxIndex(tbl); idx != 3 {
			t.Fatalf("bad index: %d (%s)", idx, tbl)
		}
	}
}

func TestStateStore_EnsureService(t *testing.T) {
	s := testStateStore(t)

	// Fetching services for a node with none returns nil
	if idx, res, err := s.NodeServices("node1"); err != nil || res != nil || idx != 0 {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Register the nodes
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

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
	idx, out, err := s.NodeServices("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Highest index for the result set was returned
	if idx != 20 {
		t.Fatalf("bad index: %d", idx)
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

	// Index tables were updated
	if idx := s.maxIndex("services"); idx != 30 {
		t.Fatalf("bad index: %d", idx)
	}

	// Update a service registration
	ns1.Address = "1.1.1.2"
	if err := s.EnsureService(40, "node1", ns1); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the service again and ensure it matches
	idx, out, err = s.NodeServices("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 40 {
		t.Fatalf("bad index: %d", idx)
	}
	if out == nil || len(out.Services) == 0 {
		t.Fatalf("bad: %#v", out)
	}
	if svc, ok := out.Services["service1"]; !ok || svc.Address != "1.1.1.2" {
		t.Fatalf("bad: %#v", svc)
	}
}

func TestStateStore_DeleteService(t *testing.T) {
	s := testStateStore(t)

	// Register a node with one service and a check
	testRegisterNode(t, s, 1, "node1")
	testRegisterService(t, s, 2, "node1", "service1")
	testRegisterCheck(t, s, 3, "node1", "service1", "check1", structs.HealthPassing)

	// Delete the service
	if err := s.DeleteService(4, "node1", "service1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Service doesn't exist.
	_, ns, err := s.NodeServices("node1")
	if err != nil || ns == nil || len(ns.Services) != 0 {
		t.Fatalf("bad: %#v (err: %#v)", ns, err)
	}

	// Check doesn't exist. Check using the raw DB so we can test
	// that it actually is removed in the state store.
	tx := s.db.Txn(false)
	defer tx.Abort()
	check, err := tx.First("checks", "id", "node1", "check1")
	if err != nil || check != nil {
		t.Fatalf("bad: %#v (err: %s)", check, err)
	}

	// Index tables were updated
	if idx := s.maxIndex("services"); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
	if idx := s.maxIndex("checks"); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_EnsureCheck(t *testing.T) {
	s := testStateStore(t)

	// Create a check associated with the node
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

	// Creating a check without a node returns error
	if err := s.EnsureCheck(1, check); err != ErrMissingNode {
		t.Fatalf("expected %#v, got: %#v", ErrMissingNode, err)
	}

	// Register the node
	testRegisterNode(t, s, 1, "node1")

	// Creating a check with a bad services returns error
	if err := s.EnsureCheck(1, check); err != ErrMissingService {
		t.Fatalf("expected: %#v, got: %#v", ErrMissingService, err)
	}

	// Register the service
	testRegisterService(t, s, 2, "node1", "service1")

	// Inserting the check with the prerequisites succeeds
	if err := s.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the check and make sure it matches
	idx, checks, err := s.NodeChecks("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(checks) != 1 {
		t.Fatalf("wrong number of checks: %d", len(checks))
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %#v", checks[0])
	}

	// Modify the health check
	check.Output = "bbb"
	if err := s.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that we successfully updated
	idx, checks, err = s.NodeChecks("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(checks) != 1 {
		t.Fatalf("wrong number of checks: %d", len(checks))
	}
	if checks[0].Output != "bbb" {
		t.Fatalf("wrong check output: %#v", checks[0])
	}
	if checks[0].CreateIndex != 3 || checks[0].ModifyIndex != 4 {
		t.Fatalf("bad index: %#v", checks[0])
	}

	// Index tables were updated
	if idx := s.maxIndex("checks"); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_ServiceChecks(t *testing.T) {
	s := testStateStore(t)

	// Create the first node and service with some checks
	testRegisterNode(t, s, 0, "node1")
	testRegisterService(t, s, 1, "node1", "service1")
	testRegisterCheck(t, s, 2, "node1", "service1", "check1", structs.HealthPassing)
	testRegisterCheck(t, s, 3, "node1", "service1", "check2", structs.HealthPassing)

	// Create a second node/service with a different set of checks
	testRegisterNode(t, s, 4, "node2")
	testRegisterService(t, s, 5, "node2", "service2")
	testRegisterCheck(t, s, 6, "node2", "service2", "check3", structs.HealthPassing)

	// Try querying for all checks associated with service1
	idx, checks, err := s.ServiceChecks("service1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(checks) != 2 || checks[0].CheckID != "check1" || checks[1].CheckID != "check2" {
		t.Fatalf("bad checks: %#v", checks)
	}
}

func TestStateStore_DeleteCheck(t *testing.T) {
	s := testStateStore(t)

	// Register a node and a node-level health check
	testRegisterNode(t, s, 1, "node1")
	testRegisterCheck(t, s, 2, "node1", "", "check1", structs.HealthPassing)

	// Delete the check
	if err := s.DeleteCheck(3, "node1", "check1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check is gone
	_, checks, err := s.NodeChecks("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(checks) != 0 {
		t.Fatalf("bad: %#v", checks)
	}

	// Index tables were updated
	if idx := s.maxIndex("checks"); idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_ChecksInState(t *testing.T) {
	s := testStateStore(t)

	// Register a node with checks in varied states
	testRegisterNode(t, s, 0, "node1")
	testRegisterCheck(t, s, 1, "node1", "", "check1", structs.HealthPassing)
	testRegisterCheck(t, s, 2, "node1", "", "check2", structs.HealthCritical)
	testRegisterCheck(t, s, 3, "node1", "", "check3", structs.HealthPassing)

	// Query the state store for passing checks.
	_, results, err := s.ChecksInState(structs.HealthPassing)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure we only get the checks which match the state
	if n := len(results); n != 2 {
		t.Fatalf("expected 2 checks, got: %d", n)
	}
	if results[0].CheckID != "check1" || results[1].CheckID != "check3" {
		t.Fatalf("bad: %#v", results)
	}

	// HealthAny just returns everything.
	_, results, err = s.ChecksInState(structs.HealthAny)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(results); n != 3 {
		t.Fatalf("expected 3 checks, got: %d", n)
	}
}

func TestStateStore_CheckServiceNodes(t *testing.T) {
	s := testStateStore(t)

	// Querying with no matches gives an empty response
	idx, results, err := s.CheckServiceNodes("service1")
	if idx != 0 || results != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, results, err)
	}

	// Register some nodes
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	// Register node-level checks. These should not be returned
	// in the final result.
	testRegisterCheck(t, s, 2, "node1", "", "check1", structs.HealthPassing)
	testRegisterCheck(t, s, 3, "node2", "", "check2", structs.HealthPassing)

	// Register a service against the nodes
	testRegisterService(t, s, 4, "node1", "service1")
	testRegisterService(t, s, 5, "node2", "service2")

	// Register checks against the services
	testRegisterCheck(t, s, 6, "node1", "service1", "check3", structs.HealthPassing)
	testRegisterCheck(t, s, 7, "node2", "service2", "check4", structs.HealthPassing)

	// Query the state store for nodes and checks which
	// have been registered with a specific service.
	idx, results, err = s.CheckServiceNodes("service1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the index returned matches the result set. The index
	// should be the highest observed from the result, in this case
	// this comes from the check registration.
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	// Make sure we get the expected result
	if n := len(results); n != 1 {
		t.Fatalf("expected 1 result, got: %d", n)
	}
	csn := results[0]
	if csn.Node == nil || csn.Service == nil || len(csn.Checks) != 1 {
		t.Fatalf("bad output: %#v", csn)
	}

	// Node updates alter the returned index
	testRegisterNode(t, s, 8, "node1")
	idx, results, err = s.CheckServiceNodes("service1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 8 {
		t.Fatalf("bad index: %d", idx)
	}

	// Service updates alter the returned index
	testRegisterService(t, s, 9, "node1", "service1")
	idx, results, err = s.CheckServiceNodes("service1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 9 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check updates alter the returned index
	testRegisterCheck(t, s, 10, "node1", "service1", "check1", structs.HealthCritical)
	idx, results, err = s.CheckServiceNodes("service1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 10 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_NodeInfo_NodeDump(t *testing.T) {
	s := testStateStore(t)

	// Generating a node dump that matches nothing returns empty
	idx, dump, err := s.NodeInfo("node1")
	if idx != 0 || dump != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, dump, err)
	}
	idx, dump, err = s.NodeDump()
	if idx != 0 || dump != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, dump, err)
	}

	// Register some nodes
	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")

	// Register services against them
	testRegisterService(t, s, 2, "node1", "service1")
	testRegisterService(t, s, 3, "node1", "service2")
	testRegisterService(t, s, 4, "node2", "service1")
	testRegisterService(t, s, 5, "node2", "service2")

	// Register service-level checks
	testRegisterCheck(t, s, 6, "node1", "service1", "check1", structs.HealthPassing)
	testRegisterCheck(t, s, 7, "node2", "service1", "check1", structs.HealthPassing)

	// Register node-level checks
	testRegisterCheck(t, s, 8, "node1", "", "check2", structs.HealthPassing)
	testRegisterCheck(t, s, 9, "node2", "", "check2", structs.HealthPassing)

	// Check that our result matches what we expect.
	expect := structs.NodeDump{
		&structs.NodeInfo{
			Node: "node1",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check1",
					ServiceID:   "service1",
					ServiceName: "service1",
					Status:      structs.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 6,
						ModifyIndex: 6,
					},
				},
				&structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check2",
					ServiceID:   "",
					ServiceName: "",
					Status:      structs.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 8,
						ModifyIndex: 8,
					},
				},
			},
			Services: []*structs.NodeService{
				&structs.NodeService{
					ID:      "service1",
					Service: "service1",
					Address: "1.1.1.1",
					Port:    1111,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 2,
						ModifyIndex: 2,
					},
				},
				&structs.NodeService{
					ID:      "service2",
					Service: "service2",
					Address: "1.1.1.1",
					Port:    1111,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 3,
						ModifyIndex: 3,
					},
				},
			},
		},
		&structs.NodeInfo{
			Node: "node2",
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:        "node2",
					CheckID:     "check1",
					ServiceID:   "service1",
					ServiceName: "service1",
					Status:      structs.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 7,
						ModifyIndex: 7,
					},
				},
				&structs.HealthCheck{
					Node:        "node2",
					CheckID:     "check2",
					ServiceID:   "",
					ServiceName: "",
					Status:      structs.HealthPassing,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 9,
						ModifyIndex: 9,
					},
				},
			},
			Services: []*structs.NodeService{
				&structs.NodeService{
					ID:      "service1",
					Service: "service1",
					Address: "1.1.1.1",
					Port:    1111,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 4,
						ModifyIndex: 4,
					},
				},
				&structs.NodeService{
					ID:      "service2",
					Service: "service2",
					Address: "1.1.1.1",
					Port:    1111,
					RaftIndex: structs.RaftIndex{
						CreateIndex: 5,
						ModifyIndex: 5,
					},
				},
			},
		},
	}

	// Get a dump of just a single node
	idx, dump, err = s.NodeInfo("node1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 8 {
		t.Fatalf("bad index: %d", idx)
	}
	if len(dump) != 1 || !reflect.DeepEqual(dump[0], expect[0]) {
		t.Fatalf("bad: %#v", dump)
	}

	// Generate a dump of all the nodes
	idx, dump, err = s.NodeDump()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 9 {
		t.Fatalf("bad index: %d", 9)
	}
	if !reflect.DeepEqual(dump, expect) {
		t.Fatalf("bad: %#v", dump[0].Services[0])
	}
}

func TestStateStore_KVSSet(t *testing.T) {
	s := testStateStore(t)

	// Write a new K/V entry to the store
	entry := &structs.DirEntry{
		Key:   "foo",
		Value: []byte("bar"),
	}
	if err := s.KVSSet(1, entry); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the K/V entry again
	result, err := s.KVSGet("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result == nil {
		t.Fatalf("expected k/v pair, got nothing")
	}

	// Check that the index was injected into the result
	if result.CreateIndex != 1 || result.ModifyIndex != 1 {
		t.Fatalf("bad index: %d, %d", result.CreateIndex, result.ModifyIndex)
	}

	// Check that the value matches
	if v := string(result.Value); v != "bar" {
		t.Fatalf("expected 'bar', got: '%s'", v)
	}

	// Updating the entry works and changes the index
	update := &structs.DirEntry{
		Key:   "foo",
		Value: []byte("baz"),
	}
	if err := s.KVSSet(2, update); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Fetch the kv pair and check
	result, err = s.KVSGet("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result.CreateIndex != 1 || result.ModifyIndex != 2 {
		t.Fatalf("bad index: %d, %d", result.CreateIndex, result.ModifyIndex)
	}
	if v := string(result.Value); v != "baz" {
		t.Fatalf("expected 'baz', got '%s'", v)
	}
}

func TestStateStore_KVSDelete(t *testing.T) {
	s := testStateStore(t)

	// Create some KV pairs
	testSetKey(t, s, 1, "foo", "foo")
	testSetKey(t, s, 2, "foo/bar", "bar")

	// Call a delete on a specific key
	if err := s.KVSDelete(3, "foo"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// The entry was removed from the state store
	tx := s.db.Txn(false)
	defer tx.Abort()
	e, err := tx.First("kvs", "id", "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if e != nil {
		t.Fatalf("expected kvs entry to be deleted, got: %#v", e)
	}

	// Try fetching the other keys to ensure they still exist
	e, err = tx.First("kvs", "id", "foo/bar")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if e == nil || string(e.(*structs.DirEntry).Value) != "bar" {
		t.Fatalf("bad kvs entry: %#v", e)
	}

	// Check that the index table was updated
	if idx := s.maxIndex("kvs"); idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestStateStore_KVSList(t *testing.T) {
	s := testStateStore(t)

	// Listing an empty KVS returns nothing
	idx, keys, err := s.KVSList("")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}
	if keys != nil {
		t.Fatalf("expected nil, got: %#v", keys)
	}

	// Create some KVS entries
	testSetKey(t, s, 1, "foo", "foo")
	testSetKey(t, s, 2, "foo/bar", "bar")
	testSetKey(t, s, 3, "foo/bar/zip", "zip")
	testSetKey(t, s, 4, "foo/bar/zip/zorp", "zorp")
	testSetKey(t, s, 5, "foo/bar/baz", "baz")

	// List out all of the keys
	idx, keys, err = s.KVSList("")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the index
	if idx != 5 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check that all of the keys were returned
	if n := len(keys); n != 5 {
		t.Fatalf("expected 5 kvs entries, got: %d", n)
	}

	// Try listing with a provided prefix
	idx, keys, err = s.KVSList("foo/bar/zip")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check that only the keys in the prefix were returned
	if n := len(keys); n != 2 {
		t.Fatalf("expected 2 kvs entries, got: %d", n)
	}
	if keys[0] != "foo/bar/zip" || keys[1] != "foo/bar/zip/zorp" {
		t.Fatalf("bad: %#v", keys)
	}
}

func TestStateStore_KVSListKeys(t *testing.T) {
	s := testStateStore(t)

	// Create some keys
	testSetKey(t, s, 1, "foo", "foo")
	testSetKey(t, s, 2, "foo/bar", "bar")
	testSetKey(t, s, 3, "foo/bar/baz", "baz")
	testSetKey(t, s, 4, "foo/bar/zip", "zip")
	testSetKey(t, s, 5, "foo/bar/zip/zam", "zam")
	testSetKey(t, s, 6, "foo/bar/zip/zorp", "zorp")

	// Query using a prefix and pass a separator
	idx, keys, err := s.KVSListKeys("foo/bar/", "/")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	// Subset of the keys was returned
	expect := []string{"foo/bar/baz", "foo/bar/zip", "foo/bar/zip/"}
	if !reflect.DeepEqual(keys, expect) {
		t.Fatalf("bad keys: %#v", keys)
	}

	// Listing keys with no separator returns everything.
	idx, keys, err = s.KVSListKeys("foo", "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	expect = []string{"foo", "foo/bar", "foo/bar/baz", "foo/bar/zip",
		"foo/bar/zip/zam", "foo/bar/zip/zorp"}
	if !reflect.DeepEqual(keys, expect) {
		t.Fatalf("bad keys: %#v", keys)
	}
}

func TestStateStore_KVSDeleteCAS(t *testing.T) {
	s := testStateStore(t)

	// Create some KV entries
	testSetKey(t, s, 1, "foo", "foo")
	testSetKey(t, s, 2, "bar", "bar")
	testSetKey(t, s, 3, "baz", "baz")

	// Do a CAS delete with an index lower than the entry
	ok, err := s.KVSDeleteCAS(4, 1, "bar")
	if ok || err != nil {
		t.Fatalf("expected (false, nil), got: (%v, %#v)", ok, err)
	}

	// Check that the index is untouched and the entry
	// has not been deleted.
	if idx := s.maxIndex("kvs"); idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	e, err := s.KVSGet("foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if e == nil {
		t.Fatalf("expected a kvs entry, got nil")
	}

	// Do another CAS delete, this time with the correct index
	// which should cause the delete to take place.
	ok, err = s.KVSDeleteCAS(4, 2, "bar")
	if !ok || err != nil {
		t.Fatalf("expected (true, nil), got: (%v, %#v)", ok, err)
	}

	// Entry was deleted and index was updated
	if idx := s.maxIndex("kvs"); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
	e, err = s.KVSGet("bar")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if e != nil {
		t.Fatalf("entry should be deleted")
	}
}

func TestStateStore_KVSSetCAS(t *testing.T) {
	s := testStateStore(t)

	// Doing a CAS with ModifyIndex != 0 and no existing entry
	// is a no-op.
	entry := &structs.DirEntry{
		Key:   "foo",
		Value: []byte("foo"),
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}
	ok, err := s.KVSSetCAS(2, entry)
	if ok || err != nil {
		t.Fatalf("expected (false, nil), got: (%#v, %#v)", ok, err)
	}

	// Check that nothing was actually stored
	tx := s.db.Txn(false)
	if e, err := tx.First("kvs", "id", "foo"); e != nil || err != nil {
		t.Fatalf("expected (nil, nil), got: (%#v, %#v)", e, err)
	}
	tx.Abort()

	// Index was not updated
	if idx := s.maxIndex("kvs"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Doing a CAS with a ModifyIndex of zero when no entry exists
	// performs the set and saves into the state store.
	entry = &structs.DirEntry{
		Key:   "foo",
		Value: []byte("foo"),
		RaftIndex: structs.RaftIndex{
			CreateIndex: 0,
			ModifyIndex: 0,
		},
	}
	ok, err = s.KVSSetCAS(2, entry)
	if !ok || err != nil {
		t.Fatalf("expected (true, nil), got: (%#v, %#v)", ok, err)
	}

	// Entry was inserted
	tx = s.db.Txn(false)
	if e, err := tx.First("kvs", "id", "foo"); e == nil || err != nil {
		t.Fatalf("expected kvs to exist, got: (%#v, %#v)", e, err)
	}
	tx.Abort()

	// Index was updated
	if idx := s.maxIndex("kvs"); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	// Doing a CAS with a ModifyIndex which does not match the current
	// index does not do anything.
	entry = &structs.DirEntry{
		Key:   "foo",
		Value: []byte("bar"),
		RaftIndex: structs.RaftIndex{
			CreateIndex: 3,
			ModifyIndex: 3,
		},
	}
	ok, err = s.KVSSetCAS(3, entry)
	if ok || err != nil {
		t.Fatalf("expected (false, nil), got: (%#v, %#v)", ok, err)
	}

	// Entry was not updated in the store
	tx = s.db.Txn(false)
	e, err := tx.First("kvs", "id", "foo")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	result, ok := e.(*structs.DirEntry)
	if !ok || result.CreateIndex != 2 ||
		result.ModifyIndex != 2 || string(result.Value) != "foo" {
		t.Fatalf("bad: %#v", result)
	}

	// Index was not modified
	if idx := s.maxIndex("kvs"); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
}
