package consul

import (
	"sort"
	"testing"
)

func TestEnsureNode(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	found, addr := store.GetNode("foo")
	if !found || addr != "127.0.0.1" {
		t.Fatalf("Bad: %v %v", found, addr)
	}

	if err := store.EnsureNode("foo", "127.0.0.2"); err != nil {
		t.Fatalf("err: %v")
	}

	found, addr = store.GetNode("foo")
	if !found || addr != "127.0.0.2" {
		t.Fatalf("Bad: %v %v", found, addr)
	}
}

func TestGetNodes(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode("bar", "127.0.0.2"); err != nil {
		t.Fatalf("err: %v")
	}

	nodes := store.Nodes()
	if len(nodes) != 4 {
		t.Fatalf("Bad: %v", nodes)
	}
	if nodes[2] != "foo" && nodes[0] != "bar" {
		t.Fatalf("Bad: %v", nodes)
	}
}

func TestEnsureService(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api", "api", "", 5000); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api", "api", "", 5001); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "db", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v", err)
	}

	services := store.NodeServices("foo")

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
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api1", "api", "", 5000); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api2", "api", "", 5001); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api3", "api", "", 5002); err != nil {
		t.Fatalf("err: %v", err)
	}

	services := store.NodeServices("foo")

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
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api", "api", "", 5000); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.DeleteNodeService("foo", "api"); err != nil {
		t.Fatalf("err: %v", err)
	}

	services := store.NodeServices("foo")
	_, ok := services.Services["api"]
	if ok {
		t.Fatalf("has api: %#v", services)
	}
}

func TestDeleteNodeService_One(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api", "api", "", 5000); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api2", "api", "", 5001); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.DeleteNodeService("foo", "api"); err != nil {
		t.Fatalf("err: %v", err)
	}

	services := store.NodeServices("foo")
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
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "api", "api", "", 5000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.DeleteNode("foo"); err != nil {
		t.Fatalf("err: %v")
	}

	services := store.NodeServices("foo")
	_, ok := services.Services["api"]
	if ok {
		t.Fatalf("has api: %#v", services)
	}

	found, _ := store.GetNode("foo")
	if found {
		t.Fatalf("found node")
	}
}

func TestGetServices(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode("bar", "127.0.0.2"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "api", "api", "", 5000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "db", "slave", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	services := store.Services()

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
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode("bar", "127.0.0.2"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "api", "api", "", 5000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "api", "api", "", 5000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "db", "slave", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db2", "db", "slave", 8001); err != nil {
		t.Fatalf("err: %v")
	}

	nodes := store.ServiceNodes("db")
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
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode("bar", "127.0.0.2"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db2", "db", "slave", 8001); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "db", "slave", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	nodes := store.ServiceTagNodes("db", "master")
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
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureNode("bar", "127.0.0.2"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db2", "db", "slave", 8001); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "db", "slave", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	// Take a snapshot
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v")
	}
	defer snap.Close()

	// Check snapshot has old values
	nodes := snap.Nodes()
	if len(nodes) != 4 {
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

	// Make some changes!
	if err := store.EnsureService("foo", "db", "db", "slave", 8000); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService("bar", "db", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureNode("baz", "127.0.0.3"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check snapshot has old values
	nodes = snap.Nodes()
	if len(nodes) != 4 {
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
}
