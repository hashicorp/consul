// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
)

func TestNewNodeIDFromConfig(t *testing.T) {
	logger := hclog.New(nil)
	tmpDir := testutil.TempDir(t, "")
	cfg := &config.RuntimeConfig{
		DataDir: tmpDir,
	}

	var randomNodeID types.NodeID
	t.Run("a new ID is generated when none is specified", func(t *testing.T) {
		var err error
		randomNodeID, err = newNodeIDFromConfig(cfg, logger)
		require.NoError(t, err)

		_, err = uuid.ParseUUID(string(randomNodeID))
		require.NoError(t, err)
	})

	t.Run("running again should get the NodeID that was persisted to disk", func(t *testing.T) {
		nodeID, err := newNodeIDFromConfig(cfg, logger)
		require.NoError(t, err)
		require.NotEqual(t, nodeID, "")
		require.Equal(t, nodeID, randomNodeID)
	})

	t.Run("invalid NodeID in config", func(t *testing.T) {
		cfg.NodeID = "nope"
		_, err := newNodeIDFromConfig(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "specified NodeID is invalid")
	})

	t.Run("valid NodeID in config", func(t *testing.T) {
		newID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		cfg.NodeID = types.NodeID(strings.ToUpper(newID))
		nodeID, err := newNodeIDFromConfig(cfg, logger)
		require.NoError(t, err)
		require.Equal(t, string(nodeID), newID)
	})

	t.Run("invalid NodeID in file", func(t *testing.T) {
		cfg.NodeID = ""
		filename := filepath.Join(cfg.DataDir, "node-id")
		err := os.WriteFile(filename, []byte("adf4238a!882b!9ddc!4a9d!5b6758e4159e"), 0600)
		require.NoError(t, err)

		_, err = newNodeIDFromConfig(cfg, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("NodeID in %s is invalid", filename))
	})

	t.Run("valid NodeID in file", func(t *testing.T) {
		cfg.NodeID = ""
		filename := filepath.Join(cfg.DataDir, "node-id")
		err := os.WriteFile(filename, []byte("ADF4238a-882b-9ddc-4a9d-5b6758e4159e"), 0600)
		require.NoError(t, err)

		nodeID, err := newNodeIDFromConfig(cfg, logger)
		require.NoError(t, err)
		require.Equal(t, string(nodeID), "adf4238a-882b-9ddc-4a9d-5b6758e4159e")
	})
}

func TestMakeNodeID(t *testing.T) {
	logger := hclog.New(nil)

	var randomID string
	t.Run("Random ID when HostNodeID is disabled", func(t *testing.T) {
		var err error
		randomID, err = makeNodeID(logger, true)
		require.NoError(t, err)

		_, err = uuid.ParseUUID(randomID)
		require.NoError(t, err)

		another, err := makeNodeID(logger, true)
		require.NoError(t, err)
		require.NotEqual(t, randomID, another)
	})

	t.Run("host-based ID when HostNodeID is enabled", func(t *testing.T) {
		id, err := makeNodeID(logger, false)
		require.NoError(t, err)
		require.NotEqual(t, randomID, id)

		another, err := makeNodeID(logger, false)
		require.NoError(t, err)
		require.Equal(t, id, another)
	})
}
