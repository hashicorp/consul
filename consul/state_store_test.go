package consul

import (
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
)

func testStateStore() (*StateStore, error) {
	return NewStateStore(nil, os.Stderr)
}

func TestEnsureRegistration(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	reg := &structs.RegisterRequest{
		Node:    "foo",
		Address: "127.0.0.1",
		Service: &structs.NodeService{"api", "api", nil, "", 5000},
		Check: &structs.HealthCheck{
			Node:      "foo",
			CheckID:   "api",
			Name:      "Can connect",
			Status:    structs.HealthPassing,
			ServiceID: "api",
		},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{
				Node:      "foo",
				CheckID:   "api-cache",
				Name:      "Can cache stuff",
				Status:    structs.HealthPassing,
				ServiceID: "api",
			},
		},
	}

	if err := store.EnsureRegistration(13, reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, found, addr := store.GetNode("foo")
	if idx != 13 || !found || addr != "127.0.0.1" {
		t.Fatalf("Bad: %v %v %v", idx, found, addr)
	}

	idx, services := store.NodeServices("foo")
	if idx != 13 {
		t.Fatalf("bad: %v", idx)
	}

	entry, ok := services.Services["api"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if len(entry.Tags) != 0 || entry.Port != 5000 {
		t.Fatalf("Bad entry: %#v", entry)
	}

	idx, checks := store.NodeChecks("foo")
	if idx != 13 {
		t.Fatalf("bad: %v", idx)
	}
	if len(checks) != 2 {
		t.Fatalf("check: %#v", checks)
	}
}

func TestEnsureNode(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, found, addr := store.GetNode("foo")
	if idx != 3 || !found || addr != "127.0.0.1" {
		t.Fatalf("Bad: %v %v %v", idx, found, addr)
	}

	if err := store.EnsureNode(4, structs.Node{"foo", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
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
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureNode(41, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
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
		b.Fatalf("err: %v", err)
	}

	if err := store.EnsureNode(101, structs.Node{"bar", "127.0.0.2"}); err != nil {
		b.Fatalf("err: %v", err)
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

	if err := store.EnsureService(11, "foo", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", nil, "", 5001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(13, "foo", &structs.NodeService{"db", "db", []string{"master"}, "", 8000}); err != nil {
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
	if len(entry.Tags) != 0 || entry.Port != 5001 {
		t.Fatalf("Bad entry: %#v", entry)
	}

	entry, ok = services.Services["db"]
	if !ok {
		t.Fatalf("missing db: %#v", services)
	}
	if !strContains(entry.Tags, "master") || entry.Port != 8000 {
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

	if err := store.EnsureService(11, "foo", &structs.NodeService{"api1", "api", nil, "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api2", "api", nil, "", 5001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(13, "foo", &structs.NodeService{"api3", "api", nil, "", 5002}); err != nil {
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
	if len(entry.Tags) != 0 || entry.Port != 5000 {
		t.Fatalf("Bad entry: %#v", entry)
	}

	entry, ok = services.Services["api2"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if len(entry.Tags) != 0 || entry.Port != 5001 {
		t.Fatalf("Bad entry: %#v", entry)
	}

	entry, ok = services.Services["api3"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if len(entry.Tags) != 0 || entry.Port != 5002 {
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

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
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

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(13, "foo", &structs.NodeService{"api2", "api", nil, "", 5001}); err != nil {
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
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(21, "foo", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
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
	if services != nil {
		t.Fatalf("has services: %#v", services)
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
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureNode(31, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(32, "foo", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(33, "foo", &structs.NodeService{"db", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(34, "bar", &structs.NodeService{"db", "db", []string{"slave"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, services := store.Services()
	if idx != 34 {
		t.Fatalf("bad: %v", idx)
	}

	tags, ok := services["api"]
	if !ok {
		t.Fatalf("missing api: %#v", services)
	}
	if len(tags) != 0 {
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
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureNode(11, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(13, "bar", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(14, "foo", &structs.NodeService{"db", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(15, "bar", &structs.NodeService{"db", "db", []string{"slave"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(16, "bar", &structs.NodeService{"db2", "db", []string{"slave"}, "", 8001}); err != nil {
		t.Fatalf("err: %v", err)
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
	if !strContains(nodes[0].ServiceTags, "master") {
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
	if !strContains(nodes[1].ServiceTags, "slave") {
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
	if !strContains(nodes[2].ServiceTags, "slave") {
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
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureNode(16, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(17, "foo", &structs.NodeService{"db", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(18, "foo", &structs.NodeService{"db2", "db", []string{"slave"}, "", 8001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(19, "bar", &structs.NodeService{"db", "db", []string{"slave"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
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
	if !strContains(nodes[0].ServiceTags, "master") {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}
}

func TestServiceTagNodes_MultipleTags(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(15, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureNode(16, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(17, "foo", &structs.NodeService{"db", "db", []string{"master", "v2"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(18, "foo", &structs.NodeService{"db2", "db", []string{"slave", "v2", "dev"}, "", 8001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(19, "bar", &structs.NodeService{"db", "db", []string{"slave", "v2"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
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
	if !strContains(nodes[0].ServiceTags, "master") {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServicePort != 8000 {
		t.Fatalf("bad: %v", nodes)
	}

	idx, nodes = store.ServiceTagNodes("db", "v2")
	if idx != 19 {
		t.Fatalf("bad: %v", idx)
	}
	if len(nodes) != 3 {
		t.Fatalf("bad: %v", nodes)
	}

	idx, nodes = store.ServiceTagNodes("db", "dev")
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
	if !strContains(nodes[0].ServiceTags, "dev") {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].ServicePort != 8001 {
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
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureNode(9, structs.Node{"bar", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(10, "foo", &structs.NodeService{"db", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(11, "foo", &structs.NodeService{"db2", "db", []string{"slave"}, "", 8001}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService(12, "bar", &structs.NodeService{"db", "db", []string{"slave"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db",
	}
	if err := store.EnsureCheck(13, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add some KVS entries
	d := &structs.DirEntry{Key: "/web/a", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(14, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/b", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(15, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/c", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(16, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Create a tombstone
	// TODO: Change to /web/c causes failure?
	if err := store.KVSDelete(17, "/web/a"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add some sessions
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	if err := store.SessionCreate(18, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	session = &structs.Session{ID: generateUUID(), Node: "bar"}
	if err := store.SessionCreate(19, session); err != nil {
		t.Fatalf("err: %v", err)
	}
	d.Session = session.ID
	if ok, err := store.KVSLock(20, d); err != nil || !ok {
		t.Fatalf("err: %v", err)
	}
	session = &structs.Session{ID: generateUUID(), Node: "bar", TTL: "60s"}
	if err := store.SessionCreate(21, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	a1 := &structs.ACL{
		ID:   generateUUID(),
		Name: "User token",
		Type: structs.ACLTypeClient,
	}
	if err := store.ACLSet(21, a1); err != nil {
		t.Fatalf("err: %v", err)
	}

	a2 := &structs.ACL{
		ID:   generateUUID(),
		Name: "User token",
		Type: structs.ACLTypeClient,
	}
	if err := store.ACLSet(22, a2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Take a snapshot
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Close()

	// Check the last nodes
	if idx := snap.LastIndex(); idx != 22 {
		t.Fatalf("bad: %v", idx)
	}

	// Check snapshot has old values
	nodes := snap.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}

	// Ensure we get the service entries
	services := snap.NodeServices("foo")
	if !strContains(services.Services["db"].Tags, "master") {
		t.Fatalf("bad: %v", services)
	}
	if !strContains(services.Services["db2"].Tags, "slave") {
		t.Fatalf("bad: %v", services)
	}

	services = snap.NodeServices("bar")
	if !strContains(services.Services["db"].Tags, "slave") {
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

	// Check we have the entries
	streamCh := make(chan interface{}, 64)
	doneCh := make(chan struct{})
	var ents []*structs.DirEntry
	go func() {
		for {
			obj := <-streamCh
			if obj == nil {
				close(doneCh)
				return
			}
			ents = append(ents, obj.(*structs.DirEntry))
		}
	}()
	if err := snap.KVSDump(streamCh); err != nil {
		t.Fatalf("err: %v", err)
	}
	<-doneCh
	if len(ents) != 2 {
		t.Fatalf("missing KVS entries! %#v", ents)
	}

	// Check we have the tombstone entries
	streamCh = make(chan interface{}, 64)
	doneCh = make(chan struct{})
	ents = nil
	go func() {
		for {
			obj := <-streamCh
			if obj == nil {
				close(doneCh)
				return
			}
			ents = append(ents, obj.(*structs.DirEntry))
		}
	}()
	if err := snap.TombstoneDump(streamCh); err != nil {
		t.Fatalf("err: %v", err)
	}
	<-doneCh
	if len(ents) != 1 {
		t.Fatalf("missing tombstone entries!")
	}

	// Check there are 3 sessions
	sessions, err := snap.SessionList()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("missing sessions")
	}

	ttls := 0
	for _, session := range sessions {
		if session.TTL != "" {
			ttls++
		}
	}
	if ttls != 1 {
		t.Fatalf("Wrong number of sessions with TTL")
	}

	// Check for an acl
	acls, err := snap.ACLList()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(acls) != 2 {
		t.Fatalf("missing acls")
	}

	// Make some changes!
	if err := store.EnsureService(23, "foo", &structs.NodeService{"db", "db", []string{"slave"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(24, "bar", &structs.NodeService{"db", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureNode(25, structs.Node{"baz", "127.0.0.3"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	checkAfter := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthCritical,
		ServiceID: "db",
	}
	if err := store.EnsureCheck(27, checkAfter); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.KVSDelete(28, "/web/b"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nuke an ACL
	if err := store.ACLDelete(29, a1.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check snapshot has old values
	nodes = snap.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}

	// Ensure old service entries
	services = snap.NodeServices("foo")
	if !strContains(services.Services["db"].Tags, "master") {
		t.Fatalf("bad: %v", services)
	}
	if !strContains(services.Services["db2"].Tags, "slave") {
		t.Fatalf("bad: %v", services)
	}

	services = snap.NodeServices("bar")
	if !strContains(services.Services["db"].Tags, "slave") {
		t.Fatalf("bad: %v", services)
	}

	checks = snap.NodeChecks("foo")
	if len(checks) != 1 {
		t.Fatalf("bad: %v", checks)
	}
	if !reflect.DeepEqual(checks[0], check) {
		t.Fatalf("bad: %v", checks[0])
	}

	// Check we have the entries
	streamCh = make(chan interface{}, 64)
	doneCh = make(chan struct{})
	ents = nil
	go func() {
		for {
			obj := <-streamCh
			if obj == nil {
				close(doneCh)
				return
			}
			ents = append(ents, obj.(*structs.DirEntry))
		}
	}()
	if err := snap.KVSDump(streamCh); err != nil {
		t.Fatalf("err: %v", err)
	}
	<-doneCh
	if len(ents) != 2 {
		t.Fatalf("missing KVS entries!")
	}

	// Check we have the tombstone entries
	streamCh = make(chan interface{}, 64)
	doneCh = make(chan struct{})
	ents = nil
	go func() {
		for {
			obj := <-streamCh
			if obj == nil {
				close(doneCh)
				return
			}
			ents = append(ents, obj.(*structs.DirEntry))
		}
	}()
	if err := snap.TombstoneDump(streamCh); err != nil {
		t.Fatalf("err: %v", err)
	}
	<-doneCh
	if len(ents) != 1 {
		t.Fatalf("missing tombstone entries!")
	}

	// Check there are 3 sessions
	sessions, err = snap.SessionList()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("missing sessions")
	}

	// Check for an acl
	acls, err = snap.ACLList()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(acls) != 2 {
		t.Fatalf("missing acls")
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
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	check2 := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "memory",
		Name:    "memory utilization",
		Status:  structs.HealthWarning,
	}
	if err := store.EnsureCheck(4, check2); err != nil {
		t.Fatalf("err: %v", err)
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

	idx, checks = store.ChecksInState(structs.HealthAny)
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
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	check2 := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "memory",
		Name:    "memory utilization",
		Status:  structs.HealthWarning,
	}
	if err := store.EnsureCheck(4, check2); err != nil {
		t.Fatalf("err: %v", err)
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
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v", err)
	}
	check = &structs.HealthCheck{
		Node:    "foo",
		CheckID: SerfCheckID,
		Name:    SerfCheckName,
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %v", err)
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
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v", err)
	}
	check = &structs.HealthCheck{
		Node:    "foo",
		CheckID: SerfCheckID,
		Name:    SerfCheckName,
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %v", err)
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
		nil,
		"",
		0}
	if err := store.EnsureService(2, "foo", srv); err != nil {
		t.Fatalf("err: %v", err)
	}

	srv = &structs.NodeService{
		"statsite-share-stats",
		"statsite-share-stats",
		nil,
		"",
		0}
	if err := store.EnsureService(3, "foo", srv); err != nil {
		t.Fatalf("err: %v", err)
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

func TestNodeInfo(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(1, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "db",
		Name:      "Can connect",
		Status:    structs.HealthPassing,
		ServiceID: "db1",
	}
	if err := store.EnsureCheck(3, check); err != nil {
		t.Fatalf("err: %v", err)
	}
	check = &structs.HealthCheck{
		Node:    "foo",
		CheckID: SerfCheckID,
		Name:    SerfCheckName,
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(4, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, dump := store.NodeInfo("foo")
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(dump) != 1 {
		t.Fatalf("Bad: %v", dump)
	}

	info := dump[0]
	if info.Node != "foo" {
		t.Fatalf("Bad: %v", info)
	}
	if info.Services[0].ID != "db1" {
		t.Fatalf("Bad: %v", info)
	}
	if len(info.Checks) != 2 {
		t.Fatalf("Bad: %v", info)
	}
	if info.Checks[0].CheckID != "db" {
		t.Fatalf("Bad: %v", info)
	}
	if info.Checks[1].CheckID != SerfCheckID {
		t.Fatalf("Bad: %v", info)
	}
}

func TestNodeDump(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(1, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(2, "foo", &structs.NodeService{"db1", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureNode(3, structs.Node{"baz", "127.0.0.2"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(4, "baz", &structs.NodeService{"db1", "db", []string{"master"}, "", 8000}); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, dump := store.NodeDump()
	if idx != 4 {
		t.Fatalf("bad: %v", idx)
	}
	if len(dump) != 2 {
		t.Fatalf("Bad: %v", dump)
	}

	info := dump[0]
	if info.Node != "baz" {
		t.Fatalf("Bad: %v", info)
	}
	if info.Services[0].ID != "db1" {
		t.Fatalf("Bad: %v", info)
	}
	info = dump[1]
	if info.Node != "foo" {
		t.Fatalf("Bad: %v", info)
	}
	if info.Services[0].ID != "db1" {
		t.Fatalf("Bad: %v", info)
	}
}

func TestKVSSet_Watch(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	notify1 := make(chan struct{}, 1)
	notify2 := make(chan struct{}, 1)
	notify3 := make(chan struct{}, 1)

	store.WatchKV("", notify1)
	store.WatchKV("foo/", notify2)
	store.WatchKV("foo/bar", notify3)

	// Create the entry
	d := &structs.DirEntry{Key: "foo/baz", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that we've fired notify1 and notify2
	select {
	case <-notify1:
	default:
		t.Fatalf("should notify root")
	}
	select {
	case <-notify2:
	default:
		t.Fatalf("should notify foo/")
	}
	select {
	case <-notify3:
		t.Fatalf("should not notify foo/bar")
	default:
	}
}

func TestKVSSet_Get(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// Should not exist
	idx, d, err := store.KVSGet("/foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %v", idx)
	}
	if d != nil {
		t.Fatalf("bad: %v", d)
	}

	// Create the entry
	d = &structs.DirEntry{Key: "/foo", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should exist exist
	idx, d, err = store.KVSGet("/foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1000 {
		t.Fatalf("bad: %v", idx)
	}
	if d.CreateIndex != 1000 {
		t.Fatalf("bad: %v", d)
	}
	if d.ModifyIndex != 1000 {
		t.Fatalf("bad: %v", d)
	}
	if d.Key != "/foo" {
		t.Fatalf("bad: %v", d)
	}
	if d.Flags != 42 {
		t.Fatalf("bad: %v", d)
	}
	if string(d.Value) != "test" {
		t.Fatalf("bad: %v", d)
	}

	// Update the entry
	d = &structs.DirEntry{Key: "/foo", Flags: 43, Value: []byte("zip")}
	if err := store.KVSSet(1010, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should update
	idx, d, err = store.KVSGet("/foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1010 {
		t.Fatalf("bad: %v", idx)
	}
	if d.CreateIndex != 1000 {
		t.Fatalf("bad: %v", d)
	}
	if d.ModifyIndex != 1010 {
		t.Fatalf("bad: %v", d)
	}
	if d.Key != "/foo" {
		t.Fatalf("bad: %v", d)
	}
	if d.Flags != 43 {
		t.Fatalf("bad: %v", d)
	}
	if string(d.Value) != "zip" {
		t.Fatalf("bad: %v", d)
	}
}

func TestKVSDelete(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	ttl := 10 * time.Millisecond
	gran := 5 * time.Millisecond
	gc, err := NewTombstoneGC(ttl, gran)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	gc.SetEnabled(true)
	store.gc = gc

	// Create the entry
	d := &structs.DirEntry{Key: "/foo", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	notify1 := make(chan struct{}, 1)
	store.WatchKV("/", notify1)

	// Delete the entry
	if err := store.KVSDelete(1020, "/foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that we've fired notify1
	select {
	case <-notify1:
	default:
		t.Fatalf("should notify /")
	}

	// Should not exist
	idx, d, err := store.KVSGet("/foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1020 {
		t.Fatalf("bad: %v", idx)
	}
	if d != nil {
		t.Fatalf("bad: %v", d)
	}

	// Check tombstone exists
	_, res, err := store.tombstoneTable.Get("id", "/foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res == nil || res[0].(*structs.DirEntry).ModifyIndex != 1020 {
		t.Fatalf("bad: %#v", d)
	}

	// Check that we get a delete
	select {
	case idx := <-gc.ExpireCh():
		if idx != 1020 {
			t.Fatalf("bad %d", idx)
		}
	case <-time.After(20 * time.Millisecond):
		t.Fatalf("should expire")
	}
}

func TestKVSDeleteCheckAndSet(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// CAS should fail, no entry
	ok, err := store.KVSDeleteCheckAndSet(1000, "/foo", 100)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("unexpected commit")
	}

	// CAS should work, no entry
	ok, err = store.KVSDeleteCheckAndSet(1000, "/foo", 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected failure")
	}

	// Make an entry
	d := &structs.DirEntry{Key: "/foo"}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Constrain on a wrong modify time
	ok, err = store.KVSDeleteCheckAndSet(1001, "/foo", 42)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("unexpected commit")
	}

	// Constrain on a correct modify time
	ok, err = store.KVSDeleteCheckAndSet(1002, "/foo", 1000)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("expected commit")
	}
}

func TestKVSCheckAndSet(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// CAS should fail, no entry
	d := &structs.DirEntry{
		ModifyIndex: 100,
		Key:         "/foo",
		Flags:       42,
		Value:       []byte("test"),
	}
	ok, err := store.KVSCheckAndSet(1000, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("unexpected commit")
	}

	// Constrain on not-exist, should work
	d.ModifyIndex = 0
	ok, err = store.KVSCheckAndSet(1001, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("expected commit")
	}

	// Constrain on not-exist, should fail
	d.ModifyIndex = 0
	ok, err = store.KVSCheckAndSet(1002, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("unexpected commit")
	}

	// Constrain on a wrong modify time
	d.ModifyIndex = 1000
	ok, err = store.KVSCheckAndSet(1003, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("unexpected commit")
	}

	// Constrain on a correct modify time
	d.ModifyIndex = 1001
	ok, err = store.KVSCheckAndSet(1004, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("expected commit")
	}
}

func TestKVS_List(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// Should not exist
	_, idx, ents, err := store.KVSList("/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %v", idx)
	}
	if len(ents) != 0 {
		t.Fatalf("bad: %v", ents)
	}

	// Create the entries
	d := &structs.DirEntry{Key: "/web/a", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/b", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1001, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/sub/c", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1002, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should list
	_, idx, ents, err = store.KVSList("/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1002 {
		t.Fatalf("bad: %v", idx)
	}
	if len(ents) != 3 {
		t.Fatalf("bad: %v", ents)
	}

	if ents[0].Key != "/web/a" {
		t.Fatalf("bad: %v", ents[0])
	}
	if ents[1].Key != "/web/b" {
		t.Fatalf("bad: %v", ents[1])
	}
	if ents[2].Key != "/web/sub/c" {
		t.Fatalf("bad: %v", ents[2])
	}
}

func TestKVSList_TombstoneIndex(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// Create the entries
	d := &structs.DirEntry{Key: "/web/a", Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/b", Value: []byte("test")}
	if err := store.KVSSet(1001, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/c", Value: []byte("test")}
	if err := store.KVSSet(1002, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nuke the last node
	err = store.KVSDeleteTree(1003, "/web/c")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add another node
	d = &structs.DirEntry{Key: "/other", Value: []byte("test")}
	if err := store.KVSSet(1004, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// List should properly reflect tombstoned value
	tombIdx, idx, ents, err := store.KVSList("/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1004 {
		t.Fatalf("bad: %v", idx)
	}
	if tombIdx != 1003 {
		t.Fatalf("bad: %v", idx)
	}
	if len(ents) != 2 {
		t.Fatalf("bad: %v", ents)
	}
}

func TestKVS_ListKeys(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// Should not exist
	idx, keys, err := store.KVSListKeys("", "/")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 0 {
		t.Fatalf("bad: %v", keys)
	}

	// Create the entries
	d := &structs.DirEntry{Key: "/web/a", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/b", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1001, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/sub/c", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1002, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should list
	idx, keys, err = store.KVSListKeys("", "/")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1002 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 1 {
		t.Fatalf("bad: %v", keys)
	}
	if keys[0] != "/" {
		t.Fatalf("bad: %v", keys)
	}

	// Should list just web
	idx, keys, err = store.KVSListKeys("/", "/")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1002 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 1 {
		t.Fatalf("bad: %v", keys)
	}
	if keys[0] != "/web/" {
		t.Fatalf("bad: %v", keys)
	}

	// Should list a, b, sub/
	idx, keys, err = store.KVSListKeys("/web/", "/")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1002 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 3 {
		t.Fatalf("bad: %v", keys)
	}
	if keys[0] != "/web/a" {
		t.Fatalf("bad: %v", keys)
	}
	if keys[1] != "/web/b" {
		t.Fatalf("bad: %v", keys)
	}
	if keys[2] != "/web/sub/" {
		t.Fatalf("bad: %v", keys)
	}

	// Should list c
	idx, keys, err = store.KVSListKeys("/web/sub/", "/")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1002 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 1 {
		t.Fatalf("bad: %v", keys)
	}
	if keys[0] != "/web/sub/c" {
		t.Fatalf("bad: %v", keys)
	}

	// Should list all
	idx, keys, err = store.KVSListKeys("/web/", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1002 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 3 {
		t.Fatalf("bad: %v", keys)
	}
	if keys[2] != "/web/sub/c" {
		t.Fatalf("bad: %v", keys)
	}
}

func TestKVS_ListKeys_Index(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// Create the entries
	d := &structs.DirEntry{Key: "/foo/a", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/bar/b", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1001, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/baz/c", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1002, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/other/d", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1003, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, keys, err := store.KVSListKeys("/foo", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1000 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 1 {
		t.Fatalf("bad: %v", keys)
	}

	idx, keys, err = store.KVSListKeys("/ba", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1002 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 2 {
		t.Fatalf("bad: %v", keys)
	}

	idx, keys, err = store.KVSListKeys("/nope", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1003 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 0 {
		t.Fatalf("bad: %v", keys)
	}
}

func TestKVS_ListKeys_TombstoneIndex(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// Create the entries
	d := &structs.DirEntry{Key: "/foo/a", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/bar/b", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1001, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/baz/c", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1002, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/other/d", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1003, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.KVSDelete(1004, "/baz/c"); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, keys, err := store.KVSListKeys("/foo", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1000 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 1 {
		t.Fatalf("bad: %v", keys)
	}

	idx, keys, err = store.KVSListKeys("/ba", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1004 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 1 {
		t.Fatalf("bad: %v", keys)
	}

	idx, keys, err = store.KVSListKeys("/nope", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1004 {
		t.Fatalf("bad: %v", idx)
	}
	if len(keys) != 0 {
		t.Fatalf("bad: %v", keys)
	}
}

func TestKVSDeleteTree(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	ttl := 10 * time.Millisecond
	gran := 5 * time.Millisecond
	gc, err := NewTombstoneGC(ttl, gran)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	gc.SetEnabled(true)
	store.gc = gc

	notify1 := make(chan struct{}, 1)
	notify2 := make(chan struct{}, 1)
	notify3 := make(chan struct{}, 1)

	store.WatchKV("", notify1)
	store.WatchKV("/web/sub", notify2)
	store.WatchKV("/other", notify3)

	// Should not exist
	err = store.KVSDeleteTree(1000, "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the entries
	d := &structs.DirEntry{Key: "/web/a", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/b", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1001, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/sub/c", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1002, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nuke the web tree
	err = store.KVSDeleteTree(1010, "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nothing should list
	tombIdx, idx, ents, err := store.KVSList("/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1010 {
		t.Fatalf("bad: %v", idx)
	}
	if tombIdx != 1010 {
		t.Fatalf("bad: %v", idx)
	}
	if len(ents) != 0 {
		t.Fatalf("bad: %v", ents)
	}

	// Check tombstones exists
	_, res, err := store.tombstoneTable.Get("id_prefix", "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("bad: %#v", d)
	}
	for _, r := range res {
		if r.(*structs.DirEntry).ModifyIndex != 1010 {
			t.Fatalf("bad: %#v", r)
		}
	}

	// Check that we've fired notify1 and notify2
	select {
	case <-notify1:
	default:
		t.Fatalf("should notify root")
	}
	select {
	case <-notify2:
	default:
		t.Fatalf("should notify /web/sub")
	}
	select {
	case <-notify3:
		t.Fatalf("should not notify /other")
	default:
	}

	// Check that we get a delete
	select {
	case idx := <-gc.ExpireCh():
		if idx != 1010 {
			t.Fatalf("bad %d", idx)
		}
	case <-time.After(20 * time.Millisecond):
		t.Fatalf("should expire")
	}
}

func TestReapTombstones(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	ttl := 10 * time.Millisecond
	gran := 5 * time.Millisecond
	gc, err := NewTombstoneGC(ttl, gran)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	gc.SetEnabled(true)
	store.gc = gc

	// Should not exist
	err = store.KVSDeleteTree(1000, "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the entries
	d := &structs.DirEntry{Key: "/web/a", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1000, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/b", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1001, d); err != nil {
		t.Fatalf("err: %v", err)
	}
	d = &structs.DirEntry{Key: "/web/sub/c", Flags: 42, Value: []byte("test")}
	if err := store.KVSSet(1002, d); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nuke just a
	err = store.KVSDelete(1010, "/web/a")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nuke the web tree
	err = store.KVSDeleteTree(1020, "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a reap, should be a noop
	if err := store.ReapTombstones(1000); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check tombstones exists
	_, res, err := store.tombstoneTable.Get("id_prefix", "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("bad: %#v", d)
	}

	// Do a reap, should remove just /web/a
	if err := store.ReapTombstones(1010); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check tombstones exists
	_, res, err = store.tombstoneTable.Get("id_prefix", "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("bad: %#v", d)
	}

	// Do a reap, should remove them all
	if err := store.ReapTombstones(1025); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check no tombstones exists
	_, res, err = store.tombstoneTable.Get("id_prefix", "/web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("bad: %#v", d)
	}
}

func TestSessionCreate(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "bar",
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(13, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	session := &structs.Session{
		ID:     generateUUID(),
		Node:   "foo",
		Checks: []string{"bar"},
	}

	if err := store.SessionCreate(1000, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	if session.CreateIndex != 1000 {
		t.Fatalf("bad: %v", session)
	}
}

func TestSessionCreate_Invalid(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// No node registered
	session := &structs.Session{
		ID:     generateUUID(),
		Node:   "foo",
		Checks: []string{"bar"},
	}
	if err := store.SessionCreate(1000, session); err.Error() != "Missing node registration" {
		t.Fatalf("err: %v", err)
	}

	// Check not registered
	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.SessionCreate(1000, session); err.Error() != "Missing check 'bar' registration" {
		t.Fatalf("err: %v", err)
	}

	// Unhealthy check
	check := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "bar",
		Status:  structs.HealthCritical,
	}
	if err := store.EnsureCheck(13, check); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.SessionCreate(1000, session); err.Error() != "Check 'bar' is in critical state" {
		t.Fatalf("err: %v", err)
	}
}

func TestSession_Lookups(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	// Create a session
	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	session := &structs.Session{
		ID:     generateUUID(),
		Node:   "foo",
		Checks: []string{},
	}
	if err := store.SessionCreate(1000, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup by ID
	idx, s2, err := store.SessionGet(session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1000 {
		t.Fatalf("bad: %v", idx)
	}
	if !reflect.DeepEqual(s2, session) {
		t.Fatalf("bad: %#v %#v", s2, session)
	}

	// Create many sessions
	ids := []string{session.ID}
	for i := 0; i < 10; i++ {
		session := &structs.Session{
			ID:   generateUUID(),
			Node: "foo",
		}
		if err := store.SessionCreate(uint64(1000+i), session); err != nil {
			t.Fatalf("err: %v", err)
		}
		ids = append(ids, session.ID)
	}

	// List all
	idx, all, err := store.SessionList()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1009 {
		t.Fatalf("bad: %v", idx)
	}

	// Retrieve the ids
	var out []string
	for _, s := range all {
		out = append(out, s.ID)
	}

	sort.Strings(ids)
	sort.Strings(out)
	if !reflect.DeepEqual(ids, out) {
		t.Fatalf("bad: %v %v", ids, out)
	}

	// List by node
	idx, nodes, err := store.NodeSessions("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1009 {
		t.Fatalf("bad: %v", idx)
	}

	// Check again for the node list
	out = nil
	for _, s := range nodes {
		out = append(out, s.ID)
	}
	sort.Strings(out)
	if !reflect.DeepEqual(ids, out) {
		t.Fatalf("bad: %v %v", ids, out)
	}
}

func TestSessionInvalidate_CriticalHealthCheck(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "bar",
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(13, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	session := &structs.Session{
		ID:     generateUUID(),
		Node:   "foo",
		Checks: []string{"bar"},
	}
	if err := store.SessionCreate(14, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Invalidate the check
	check.Status = structs.HealthCritical
	if err := store.EnsureCheck(15, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup by ID, should be nil
	_, s2, err := store.SessionGet(session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s2 != nil {
		t.Fatalf("session should be invalidated")
	}
}

func TestSessionInvalidate_DeleteHealthCheck(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	check := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "bar",
		Status:  structs.HealthPassing,
	}
	if err := store.EnsureCheck(13, check); err != nil {
		t.Fatalf("err: %v", err)
	}

	session := &structs.Session{
		ID:     generateUUID(),
		Node:   "foo",
		Checks: []string{"bar"},
	}
	if err := store.SessionCreate(14, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the check
	if err := store.DeleteNodeCheck(15, "foo", "bar"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup by ID, should be nil
	_, s2, err := store.SessionGet(session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s2 != nil {
		t.Fatalf("session should be invalidated")
	}
}

func TestSessionInvalidate_DeleteNode(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	session := &structs.Session{
		ID:   generateUUID(),
		Node: "foo",
	}
	if err := store.SessionCreate(14, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the node
	if err := store.DeleteNode(15, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup by ID, should be nil
	_, s2, err := store.SessionGet(session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s2 != nil {
		t.Fatalf("session should be invalidated")
	}
}

func TestSessionInvalidate_DeleteNodeService(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(11, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService(12, "foo", &structs.NodeService{"api", "api", nil, "", 5000}); err != nil {
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
		t.Fatalf("err: %v", err)
	}

	session := &structs.Session{
		ID:     generateUUID(),
		Node:   "foo",
		Checks: []string{"api"},
	}
	if err := store.SessionCreate(14, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should invalidate the session
	if err := store.DeleteNodeService(15, "foo", "api"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup by ID, should be nil
	_, s2, err := store.SessionGet(session.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s2 != nil {
		t.Fatalf("session should be invalidated")
	}
}

func TestKVSLock(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	if err := store.SessionCreate(4, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lock with a non-existing keys should work
	d := &structs.DirEntry{
		Key:     "/foo",
		Flags:   42,
		Value:   []byte("test"),
		Session: session.ID,
	}
	ok, err := store.KVSLock(5, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected fail")
	}
	if d.LockIndex != 1 {
		t.Fatalf("bad: %v", d)
	}

	// Re-locking should fail
	ok, err = store.KVSLock(6, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected fail")
	}

	// Set a normal key
	k1 := &structs.DirEntry{
		Key:   "/bar",
		Flags: 0,
		Value: []byte("asdf"),
	}
	if err := store.KVSSet(7, k1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should acquire the lock
	k1.Session = session.ID
	ok, err = store.KVSLock(8, k1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected fail")
	}

	// Re-acquire should fail
	ok, err = store.KVSLock(9, k1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected fail")
	}

}

func TestKVSUnlock(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	session := &structs.Session{ID: generateUUID(), Node: "foo"}
	if err := store.SessionCreate(4, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Unlock with a non-existing keys should fail
	d := &structs.DirEntry{
		Key:     "/foo",
		Flags:   42,
		Value:   []byte("test"),
		Session: session.ID,
	}
	ok, err := store.KVSUnlock(5, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected fail")
	}

	// Lock should work
	d.Session = session.ID
	if ok, _ := store.KVSLock(6, d); !ok {
		t.Fatalf("expected lock")
	}

	// Unlock should work
	d.Session = session.ID
	ok, err = store.KVSUnlock(7, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected fail")
	}

	// Re-lock should work
	d.Session = session.ID
	if ok, err := store.KVSLock(8, d); err != nil {
		t.Fatalf("err: %v", err)
	} else if !ok {
		t.Fatalf("expected lock")
	}
	if d.LockIndex != 2 {
		t.Fatalf("bad: %v", d)
	}
}

func TestSessionInvalidate_KeyUnlock(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()
	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	session := &structs.Session{
		ID:        generateUUID(),
		Node:      "foo",
		LockDelay: 50 * time.Millisecond,
	}
	if err := store.SessionCreate(4, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lock a key with the session
	d := &structs.DirEntry{
		Key:     "/foo",
		Flags:   42,
		Value:   []byte("test"),
		Session: session.ID,
	}
	ok, err := store.KVSLock(5, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected fail")
	}

	notify1 := make(chan struct{}, 1)
	store.WatchKV("/f", notify1)

	// Delete the node
	if err := store.DeleteNode(6, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Key should be unlocked
	idx, d2, err := store.KVSGet("/foo")
	if idx != 6 {
		t.Fatalf("bad: %v", idx)
	}
	if d2.LockIndex != 1 {
		t.Fatalf("bad: %v", *d2)
	}
	if d2.Session != "" {
		t.Fatalf("bad: %v", *d2)
	}

	// Should notify of update
	select {
	case <-notify1:
	default:
		t.Fatalf("should notify /f")
	}

	// Key should have a lock delay
	expires := store.KVSLockDelay("/foo")
	if expires.Before(time.Now().Add(30 * time.Millisecond)) {
		t.Fatalf("Bad: %v", expires)
	}
}

func TestSessionInvalidate_KeyDelete(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode(3, structs.Node{"foo", "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	session := &structs.Session{
		ID:        generateUUID(),
		Node:      "foo",
		LockDelay: 50 * time.Millisecond,
		Behavior:  structs.SessionKeysDelete,
	}
	if err := store.SessionCreate(4, session); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lock a key with the session
	d := &structs.DirEntry{
		Key:     "/bar",
		Flags:   42,
		Value:   []byte("test"),
		Session: session.ID,
	}
	ok, err := store.KVSLock(5, d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected fail")
	}

	notify1 := make(chan struct{}, 1)
	store.WatchKV("/b", notify1)

	// Delete the node
	if err := store.DeleteNode(6, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Key should be deleted
	_, d2, err := store.KVSGet("/bar")
	if d2 != nil {
		t.Fatalf("unexpected undeleted key")
	}

	// Should notify of update
	select {
	case <-notify1:
	default:
		t.Fatalf("should notify /b")
	}

	// Key should have a lock delay
	expires := store.KVSLockDelay("/bar")
	if expires.Before(time.Now().Add(30 * time.Millisecond)) {
		t.Fatalf("Bad: %v", expires)
	}
}

func TestACLSet_Get(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	idx, out, err := store.ACLGet("1234")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 0 {
		t.Fatalf("bad: %v", idx)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	a := &structs.ACL{
		ID:    generateUUID(),
		Name:  "User token",
		Type:  structs.ACLTypeClient,
		Rules: "",
	}
	if err := store.ACLSet(50, a); err != nil {
		t.Fatalf("err: %v", err)
	}
	if a.CreateIndex != 50 {
		t.Fatalf("Bad: %v", a)
	}
	if a.ModifyIndex != 50 {
		t.Fatalf("Bad: %v", a)
	}
	if a.ID == "" {
		t.Fatalf("Bad: %v", a)
	}

	idx, out, err = store.ACLGet(a.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 50 {
		t.Fatalf("bad: %v", idx)
	}
	if !reflect.DeepEqual(out, a) {
		t.Fatalf("bad: %v", out)
	}

	// Update
	a.Rules = "foo bar baz"
	if err := store.ACLSet(52, a); err != nil {
		t.Fatalf("err: %v", err)
	}
	if a.CreateIndex != 50 {
		t.Fatalf("Bad: %v", a)
	}
	if a.ModifyIndex != 52 {
		t.Fatalf("Bad: %v", a)
	}

	idx, out, err = store.ACLGet(a.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 52 {
		t.Fatalf("bad: %v", idx)
	}
	if !reflect.DeepEqual(out, a) {
		t.Fatalf("bad: %v", out)
	}
}

func TestACLDelete(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	a := &structs.ACL{
		ID:    generateUUID(),
		Name:  "User token",
		Type:  structs.ACLTypeClient,
		Rules: "",
	}
	if err := store.ACLSet(50, a); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.ACLDelete(52, a.ID); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.ACLDelete(53, a.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, out, err := store.ACLGet(a.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 52 {
		t.Fatalf("bad: %v", idx)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestACLList(t *testing.T) {
	store, err := testStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	a1 := &structs.ACL{
		ID:   generateUUID(),
		Name: "User token",
		Type: structs.ACLTypeClient,
	}
	if err := store.ACLSet(50, a1); err != nil {
		t.Fatalf("err: %v", err)
	}

	a2 := &structs.ACL{
		ID:   generateUUID(),
		Name: "User token",
		Type: structs.ACLTypeClient,
	}
	if err := store.ACLSet(51, a2); err != nil {
		t.Fatalf("err: %v", err)
	}

	idx, out, err := store.ACLList()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 51 {
		t.Fatalf("bad: %v", idx)
	}
	if len(out) != 2 {
		t.Fatalf("bad: %v", out)
	}
}
