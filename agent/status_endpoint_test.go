package agent

import (
	"testing"
)

func TestStatusLeader(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	obj, err := a.srv.StatusLeader(nil, nil)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(string)
	if val == "" {
		t.Fatalf("bad addr: %v", obj)
	}
}

func TestStatusPeers(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	obj, err := a.srv.StatusPeers(nil, nil)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	peers := obj.([]string)
	if len(peers) != 1 {
		t.Fatalf("bad peers: %v", peers)
	}
}
