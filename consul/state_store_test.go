package consul

import (
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

	nodes := store.GetNodes()
	if nodes[0] != "foo" || nodes[1] != "127.0.0.1" {
		t.Fatalf("Bad: %v", nodes)
	}

	if err := store.EnsureNode("foo", "127.0.0.2"); err != nil {
		t.Fatalf("err: %v")
	}

	nodes = store.GetNodes()
	if nodes[0] != "foo" || nodes[1] != "127.0.0.2" {
		t.Fatalf("Bad: %v", nodes)
	}
}
