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

	if err := store.EnsureService("foo", "api", "", 5000); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api", "", 5001); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "db", "master", 8000); err != nil {
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

func TestDeleteNodeService(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := store.EnsureService("foo", "api", "", 5000); err != nil {
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

func TestDeleteNode(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer store.Close()

	if err := store.EnsureNode("foo", "127.0.0.1"); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "api", "", 5000); err != nil {
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

	if err := store.EnsureService("foo", "api", "", 5000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "slave", 8000); err != nil {
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

	if err := store.EnsureService("foo", "api", "", 5000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "api", "", 5000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("foo", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "slave", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	nodes := store.ServiceNodes("db")
	if len(nodes) != 2 {
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

	if nodes[1].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Address != "127.0.0.2" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].ServiceTag != "slave" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].ServicePort != 8000 {
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

	if err := store.EnsureService("foo", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "slave", 8000); err != nil {
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

	if err := store.EnsureService("foo", "db", "master", 8000); err != nil {
		t.Fatalf("err: %v")
	}

	if err := store.EnsureService("bar", "db", "slave", 8000); err != nil {
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

	services = snap.NodeServices("bar")
	if services.Services["db"].Tag != "slave" {
		t.Fatalf("bad: %v", services)
	}

	// Make some changes!
	if err := store.EnsureService("foo", "db", "slave", 8000); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := store.EnsureService("bar", "db", "master", 8000); err != nil {
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

	services = snap.NodeServices("bar")
	if services.Services["db"].Tag != "slave" {
		t.Fatalf("bad: %v", services)
	}
}
