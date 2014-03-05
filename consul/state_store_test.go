package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"os"
	"reflect"
	"sort"
	"testing"
)

func testStateStore() (*StateStore, error) {
	return NewStateStore(os.Stderr)
}

func TestEnsureNode(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v")
	}

	idx, found, addr := store.GetNode("foo")
	if idx != 3 || !found || addr != "127.0.0.1" {
		t.Fatalf("Bad: %v %v %v", idx, found, addr)
	}

	if err := store.EnsureNode(4, structs.Node{"foo", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v")
	}

	idx, found, addr = store.GetNode("foo")
	if idx != 4 || !found || addr != "127.0.0.2" {
		t.Fatalf("Bad: %v %v %v", idx, found, addr)
	}
}

func TestGetNodes(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(40, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode(41, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v")
	}

	idx, nodes := store.Nodes()
	if idx != 41 {
		t.Fatalf("idx: %v", idx)
	}
	if len(nodes) != 2 {
		t.Fatalf("Bad: %v", nodes)
	}
	if nodes[1].Node != "foo" && nodes[0].Node != "bar" {
		t.Fatalf("Bad: %v", nodes)
	}
}

func BenchmarkGetNodes(b *testing.B) {
	store, err := testStateStore()
	if err != nil {
		b.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(100, structs.Node{"foo", "127.0.0.1"}); err != nil {
		b.Fatalf("err: %v")
	}

	if err := store.EnsureNode(101, structs.Node{"bar", "127.0.0.2"}); err != nil {
		b.Fatalf("err: %v")
	}

	for i := 0; i < b.N; i++ {
		store.Nodes()
	}
}

func TestEnsureService(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(10, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(11, "foo", &structs.NodeService{"api", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", "", 5001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(13, "foo", &structs.NodeService{"db", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, services := store.NodeServices("foo")
	if idx != 13 {
		t.Fatalf("bad: %v", idx)
	}

	entry, ok := services.Services["api"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if entry.Tag != "" || entry.Port != 5001 {
		t.Fatalf("Bad entry: %#v", entry)
	}

	entry, ok = services.Services["db"]
	if !ok {
		t.Fatalf("missing db: %#v", services)
	}
	if entry.Tag != "master" || entry.Port != 8000 {
		t.Fatalf("Bad entry: %#v", entry)
	}
}

func TestEnsureService_DuplicateNode(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(10, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(11, "foo", &structs.NodeService{"api1", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api2", "api", "", 5001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(13, "foo", &structs.NodeService{"api3", "api", "", 5002}); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, services := store.NodeServices("foo")
	if idx != 13 {
		t.Fatalf("bad: %v", idx)
	}

	entry, ok := services.Services["api1"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if entry.Tag != "" || entry.Port != 5000 {
		t.Fatalf("Bad entry: %#v", entry)
	}

	entry, ok = services.Services["api2"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if entry.Tag != "" || entry.Port != 5001 {
		t.Fatalf("Bad entry: %#v", entry)
	}

	entry, ok = services.Services["api3"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if entry.Tag != "" || entry.Port != 5002 {
		t.Fatalf("Bad entry: %#v", entry)
	}
}

func TestDeleteNodeService(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(11, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "api",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "api",
	}
	if err := store.EnsureCheck(13, check); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.DeleteNodeService(14, "foo", "api"); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, services := store.NodeServices("foo")
	if idx != 14 {
		t.Fatalf("bad: %v", idx)
	}
	_, ok := services.Services["api"]
	if ok {
		t.Fatalf("has api: %#v", services)
	}

	idx, checks := store.NodeChecks("foo")
	if idx != 14 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) != 0 {
		t.Fatalf("has check: %#v", checks)
	}
}

func TestDeleteNodeService_One(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(11, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(13, "foo", &structs.NodeService{"api2", "api", "", 5001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.DeleteNodeService(14, "foo", "api"); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, services := store.NodeServices("foo")
	if idx != 14 {
		t.Fatalf("bad: %v", idx)
	}
	_, ok := services.Services["api"]
	if ok {
		t.Fatalf("has api: %#v", services)
	}
	_, ok = services.Services["api2"]
	if !ok {
		t.Fatalf("does not have api2: %#v", services)
	}
}

func TestDeleteNode(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(20, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(21, "foo", &structs.NodeService{"api", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v")
	}

	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "api",
	}
	if err := store.EnsureCheck(22, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.DeleteNode(23, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, services := store.NodeServices("foo")
	if idx != 23 {
		t.Fatalf("bad: %v", idx)
	}
	_, ok := services.Services["api"]
	if ok {
		t.Fatalf("has api: %#v", services)
	}

	idx, checks := store.NodeChecks("foo")
	if idx != 23 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) > 0 {
		t.Fatalf("has checks: %v", checks)
	}

	idx, found, _ := store.GetNode("foo")
	if idx != 23 {
		t.Fatalf("bad: %v", idx)
	}
	if found {
		t.Fatalf("found node")
	}
}

func TestGetServices(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(30, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode(31, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(32, "foo", &structs.NodeService{"api", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(33, "foo", &structs.NodeService{"db", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(34, "bar", &structs.NodeService{"db", "db", "slave", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	idx, services := store.Services()
	if idx != 34 {
		t.Fatalf("bad: %v", idx)
	}

	tags, ok := services["api"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if len(tags) != 1 || tags[0] != "" {
		t.Fatalf("Bad entry: %#v", tags)
	}

	tags, ok = services["db"]
	sort.Strings(tags)
	if !ok {
		t.Fatalf("missing db: %#v", services)
	}
	if len(tags) != 2 || tags[0] != "master" || tags[1] != "slave" {
		t.Fatalf("Bad entry: %#v", tags)
	}
}

func TestServiceNodes(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(10, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode(11, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(13, "bar", &structs.NodeService{"api", "api", "", 5000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(14, "foo", &structs.NodeService{"db", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(15, "bar", &structs.NodeService{"db", "db", "slave", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(16, "bar", &structs.NodeService{"db2", "db", "slave", 8001}); err != nil {
		t.Fatalf("err: %v")
	}

	idx, nodes := store.ServiceNodes("db")
	if idx != 16 {
		t.Fatalf("bad: %v", 16)
	}
	if len(nodes) != 3 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServiceID != "db" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServiceTag != "master" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}

	if nodes[1].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Address != "127.0.0.2" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].ServiceID != "db" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].ServiceTag != "slave" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}

	if nodes[2].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].Address != "127.0.0.2" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].ServiceID != "db2" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].ServiceTag != "slave" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].ServicePort != 8001 {
		t.Fatalf("bad: %v", nodes)
	}
}

func TestServiceTagNodes(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(15, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode(16, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(17, "foo", &structs.NodeService{"db", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(18, "foo", &structs.NodeService{"db2", "db", "slave", 8001}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(19, "bar", &structs.NodeService{"db", "db", "slave", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	idx, nodes := store.ServiceTagNodes("db", "master")
	if idx != 19 {
		t.Fatalf("bad: %v", idx)
	}
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServiceTag != "master" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}
}

func TestStoreSnapshot(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(8, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode(9, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(10, "foo", &structs.NodeService{"db", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(11, "foo", &structs.NodeService{"db2", "db", "slave", 8001}); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService(12, "bar", &structs.NodeService{"db", "db", "slave", 8000}); err != nil {
		t.Fatalf("err: %v")
	}

	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db",
	}
	if err := store.EnsureCheck(13, check); err != nil {
		t.Fatalf("err: %v")
	}

	// Take a snapshot
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v")
	}
	defer snap.Close()

	// Check the last nodes
	if idx := snap.LastIndex(); idx != 13 {
		t.Fatalf("bad: %v", idx)
	}

	// Check snapshot has old values
	nodes := snap.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}

	// Ensure we get the service entries
	services := snap.NodeServices("foo")
	if services.Services["db"].Tag != "master" {
		t.Fatalf("bad: %v", services)
	}
	if services.Services["db2"].Tag != "slave" {
		t.Fatalf("bad: %v", services)
	}

	services = snap.NodeServices("bar")
	if services.Services["db"].Tag != "slave" {
		t.Fatalf("bad: %v", services)
	}

	// Ensure we get the checks
	checks := snap.NodeChecks("foo")
	if len(checks) != 1 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %v", checks[0])
	}

	// Make some changes!
	if err := store.EnsureService(14, "foo", &structs.NodeService{"db", "db", "slave", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(15, "bar", &structs.NodeService{"db", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureNode(16, structs.Node{"baz", "127.0.0.3"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	checkAfter := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthCritical,
		ServiceID: "db",
	}
	if err := store.EnsureCheck(17, checkAfter); err != nil {
		t.Fatalf("err: %v")
	}

	// Check snapshot has old values
	nodes = snap.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}

	// Ensure old service entries
	services = snap.NodeServices("foo")
	if services.Services["db"].Tag != "master" {
		t.Fatalf("bad: %v", services)
	}
	if services.Services["db2"].Tag != "slave" {
		t.Fatalf("bad: %v", services)
	}

	services = snap.NodeServices("bar")
	if services.Services["db"].Tag != "slave" {
		t.Fatalf("bad: %v", services)
	}

	checks = snap.NodeChecks("foo")
	if len(checks) != 1 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %v", checks[0])
	}
}

func TestEnsureCheck(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(1, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v")
	}

	check2 := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "memory",
		Name:    "memory utilization",
		Status:  structs.HealthWarning,
	}
	if err := store.EnsureCheck(4, check2); err != nil {
		t.Fatalf("err: %v")
	}

	idx, checks := store.NodeChecks("foo")
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) != 2 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %v", checks[0])
	}
	if !reflect.DeepEqual(checks[1], check2) {
		t.Fatalf("bad: %v", checks[1])
	}

	idx, checks = store.ServiceChecks("db")
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) != 1 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %v", checks[0])
	}

	idx, checks = store.ChecksInState(structs.HealthPassing)
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) != 1 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %v", checks[0])
	}

	idx, checks = store.ChecksInState(structs.HealthWarning)
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) != 1 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check2) {
		t.Fatalf("bad: %v", checks[0])
	}
}

func TestDeleteNodeCheck(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(1, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v")
	}

	check2 := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "memory",
		Name:    "memory utilization",
		Status:  structs.HealthWarning,
	}
	if err := store.EnsureCheck(4, check2); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.DeleteNodeCheck(5, "foo", "db"); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, checks := store.NodeChecks("foo")
	if idx != 5 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) != 1 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check2) {
		t.Fatalf("bad: %v", checks[0])
	}
}

func TestCheckServiceNodes(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(1, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v")
	}
	check = &structs.HealthCheck{
		Node:    "foo",
		CheckID: SerfCheckID,
		Name:    SerfCheckName,
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %v")
	}

	idx, nodes := store.CheckServiceNodes("db")
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(nodes) != 1 {
		t.Fatalf("Bad: %v", nodes)
	}

	if nodes[0].Node.Node != "foo" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Service.ID != "db1" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if len(nodes[0].Checks) != 2 {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Checks[0].CheckID != "db" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Checks[1].CheckID != SerfCheckID {
		t.Fatalf("Bad: %v", nodes[0])
	}

	idx, nodes = store.CheckServiceTagNodes("db", "master")
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(nodes) != 1 {
		t.Fatalf("Bad: %v", nodes)
	}

	if nodes[0].Node.Node != "foo" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Service.ID != "db1" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if len(nodes[0].Checks) != 2 {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Checks[0].CheckID != "db" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Checks[1].CheckID != SerfCheckID {
		t.Fatalf("Bad: %v", nodes[0])
	}
}
func BenchmarkCheckServiceNodes(t *testing.B) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(1, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", "master", 8000}); err != nil {
		t.Fatalf("err: %v")
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v")
	}
	check = &structs.HealthCheck{
		Node:    "foo",
		CheckID: SerfCheckID,
		Name:    SerfCheckName,
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %v")
	}

	for i := 0; i < t.N; i++ {
		store.CheckServiceNodes("db")
	}
}

func TestSS_Register_Deregister_Query(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(1, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	srv := &structs.NodeService{
		"statsite-box-stats",
		"statsite-box-stats",
		"",
		0}
	if err := store.EnsureService(2, "foo", srv); err != nil {
		t.Fatalf("err: %v")
	}

	srv = &structs.NodeService{
		"statsite-share-stats",
		"statsite-share-stats",
		"",
		0}
	if err := store.EnsureService(3, "foo", srv); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.DeleteNode(4, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, nodes := store.CheckServiceNodes("statsite-share-stats")
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(nodes) != 0 {
		t.Fatalf("Bad: %v", nodes)
	}
}
