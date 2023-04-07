package agent

import (
	"crypto/sha512"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/shirou/gopsutil/v3/host"
)

// newNodeIDFromConfig will pull the persisted node ID, if any, or create a random one
// and persist it.
func newNodeIDFromConfig(config *config.RuntimeConfig, logger hclog.Logger) (types.NodeID, error) {
	if config.NodeID != "" {
		nodeID := strings.ToLower(string(config.NodeID))
		if _, err := uuid.ParseUUID(nodeID); err != nil {
			return "", fmt.Errorf("specified NodeID is invalid: %w", err)
		}
		return types.NodeID(nodeID), nil
	}

	// For dev mode we have no filesystem access so just make one.
	if config.DataDir == "" {
		id, err := makeNodeID(logger, config.DisableHostNodeID)
		return types.NodeID(id), err
	}

	// Load saved state, if any. Since a user could edit this, we also validate it.
	filename := filepath.Join(config.DataDir, "node-id")
	if _, err := os.Stat(filename); err == nil {
		rawID, err := os.ReadFile(filename)
		if err != nil {
			return "", err
		}

		nodeID := strings.TrimSpace(string(rawID))
		nodeID = strings.ToLower(nodeID)
		if _, err = uuid.ParseUUID(nodeID); err != nil {
			return "", fmt.Errorf("NodeID in %s is invalid: %w", filename, err)
		}
		return types.NodeID(nodeID), nil
	}

	id, err := makeNodeID(logger, config.DisableHostNodeID)
	if err != nil {
		return "", fmt.Errorf("failed to create a NodeID: %w", err)
	}
	if err := lib.EnsurePath(filename, false); err != nil {
		return "", err
	}
	if err := os.WriteFile(filename, []byte(id), 0600); err != nil {
		return "", fmt.Errorf("failed to write NodeID to disk: %w", err)
	}
	return types.NodeID(id), nil
}

// makeNodeID will try to find a host-specific ID, or else will generate a
// random ID. The returned ID will always be formatted as a GUID. We don't tell
// the caller whether this ID is random or stable since the consequences are
// high for us if this changes, so we will persist it either way. This will let
// gopsutil change implementations without affecting in-place upgrades of nodes.
func makeNodeID(logger hclog.Logger, disableHostNodeID bool) (string, error) {
	if disableHostNodeID {
		return uuid.GenerateUUID()
	}

	// Try to get a stable ID associated with the host itself.
	info, err := host.Info()
	if err != nil {
		logger.Debug("Couldn't get a unique ID from the host", "error", err)
		return uuid.GenerateUUID()
	}

	// Hash the input to make it well distributed. The reported Host UUID may be
	// similar across nodes if they are on a cloud provider or on motherboards
	// created from the same batch.
	buf := sha512.Sum512([]byte(strings.ToLower(info.HostID)))
	id := fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])

	logger.Debug("Using unique ID from host as node ID", "id", id)
	return id, nil
}
