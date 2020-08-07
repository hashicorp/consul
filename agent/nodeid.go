package agent

import (
	"crypto/sha512"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-uuid"
	"github.com/shirou/gopsutil/host"
)

// setupNodeID will pull the persisted node ID, if any, or create a random one
// and persist it.
// FIXME: move to a different file.
func (a *Agent) setupNodeID(config *config.RuntimeConfig) error {
	// If they've configured a node ID manually then just use that, as
	// long as it's valid.
	if config.NodeID != "" {
		config.NodeID = types.NodeID(strings.ToLower(string(config.NodeID)))
		if _, err := uuid.ParseUUID(string(config.NodeID)); err != nil {
			return err
		}

		return nil
	}

	// For dev mode we have no filesystem access so just make one.
	if a.config.DataDir == "" {
		id, err := a.makeNodeID()
		if err != nil {
			return err
		}

		config.NodeID = types.NodeID(id)
		return nil
	}

	// Load saved state, if any. Since a user could edit this, we also
	// validate it.
	fileID := filepath.Join(config.DataDir, "node-id")
	if _, err := os.Stat(fileID); err == nil {
		rawID, err := ioutil.ReadFile(fileID)
		if err != nil {
			return err
		}

		nodeID := strings.TrimSpace(string(rawID))
		nodeID = strings.ToLower(nodeID)
		if _, err := uuid.ParseUUID(nodeID); err != nil {
			return err
		}

		config.NodeID = types.NodeID(nodeID)
	}

	// If we still don't have a valid node ID, make one.
	if config.NodeID == "" {
		id, err := a.makeNodeID()
		if err != nil {
			return err
		}
		if err := lib.EnsurePath(fileID, false); err != nil {
			return err
		}
		if err := ioutil.WriteFile(fileID, []byte(id), 0600); err != nil {
			return err
		}

		config.NodeID = types.NodeID(id)
	}
	return nil
}

// makeNodeID will try to find a host-specific ID, or else will generate a
// random ID. The returned ID will always be formatted as a GUID. We don't tell
// the caller whether this ID is random or stable since the consequences are
// high for us if this changes, so we will persist it either way. This will let
// gopsutil change implementations without affecting in-place upgrades of nodes.
func (a *Agent) makeNodeID() (string, error) {
	// If they've disabled host-based IDs then just make a random one.
	if a.config.DisableHostNodeID {
		return a.makeRandomID()
	}

	// Try to get a stable ID associated with the host itself.
	info, err := host.Info()
	if err != nil {
		a.logger.Debug("Couldn't get a unique ID from the host", "error", err)
		return a.makeRandomID()
	}

	// Make sure the host ID parses as a UUID, since we don't have complete
	// control over this process.
	id := strings.ToLower(info.HostID)
	if _, err := uuid.ParseUUID(id); err != nil {
		a.logger.Debug("Unique ID from host isn't formatted as a UUID",
			"id", id,
			"error", err,
		)
		return a.makeRandomID()
	}

	// Hash the input to make it well distributed. The reported Host UUID may be
	// similar across nodes if they are on a cloud provider or on motherboards
	// created from the same batch.
	buf := sha512.Sum512([]byte(id))
	id = fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])

	a.logger.Debug("Using unique ID from host as node ID", "id", id)
	return id, nil
}

// makeRandomID will generate a random UUID for a node.
func (a *Agent) makeRandomID() (string, error) {
	id, err := uuid.GenerateUUID()
	if err != nil {
		return "", err
	}

	a.logger.Debug("Using random ID as node ID", "id", id)
	return id, nil
}
