package agent

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-uuid"
)

func TestSetupNodeID(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, `
		node_id = ""
	`)
	defer a.Shutdown()

	cfg := a.config

	// The auto-assigned ID should be valid.
	id := a.consulConfig().NodeID
	if _, err := uuid.ParseUUID(string(id)); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Running again should get the same ID (persisted in the file).
	cfg.NodeID = ""
	if err := a.setupNodeID(cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
	if newID := a.consulConfig().NodeID; id != newID {
		t.Fatalf("bad: %q vs %q", id, newID)
	}

	// Set an invalid ID via.Config.
	cfg.NodeID = types.NodeID("nope")
	err := a.setupNodeID(cfg)
	if err == nil || !strings.Contains(err.Error(), "uuid string is wrong length") {
		t.Fatalf("err: %v", err)
	}

	// Set a valid ID via.Config.
	newID, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	cfg.NodeID = types.NodeID(strings.ToUpper(newID))
	if err := a.setupNodeID(cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
	if id := a.consulConfig().NodeID; string(id) != newID {
		t.Fatalf("bad: %q vs. %q", id, newID)
	}

	// Set an invalid ID via the file.
	fileID := filepath.Join(cfg.DataDir, "node-id")
	if err := ioutil.WriteFile(fileID, []byte("adf4238a!882b!9ddc!4a9d!5b6758e4159e"), 0600); err != nil {
		t.Fatalf("err: %v", err)
	}
	cfg.NodeID = ""
	err = a.setupNodeID(cfg)
	if err == nil || !strings.Contains(err.Error(), "uuid is improperly formatted") {
		t.Fatalf("err: %v", err)
	}

	// Set a valid ID via the file.
	if err := ioutil.WriteFile(fileID, []byte("ADF4238a-882b-9ddc-4a9d-5b6758e4159e"), 0600); err != nil {
		t.Fatalf("err: %v", err)
	}
	cfg.NodeID = ""
	if err := a.setupNodeID(cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
	if id := a.consulConfig().NodeID; string(id) != "adf4238a-882b-9ddc-4a9d-5b6758e4159e" {
		t.Fatalf("bad: %q vs. %q", id, newID)
	}
}

func TestMakeNodeID(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, `
		node_id = ""
	`)
	defer a.Shutdown()

	// We should get a valid host-based ID initially.
	id, err := a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := uuid.ParseUUID(id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Calling again should yield a random ID by default.
	another, err := a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == another {
		t.Fatalf("bad: %s vs %s", id, another)
	}

	// Turn on host-based IDs and try again. We should get the same ID
	// each time (and a different one from the random one above).
	a.GetConfig().DisableHostNodeID = false
	id, err = a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == another {
		t.Fatalf("bad: %s vs %s", id, another)
	}

	// Calling again should yield the host-based ID.
	another, err = a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != another {
		t.Fatalf("bad: %s vs %s", id, another)
	}
}
