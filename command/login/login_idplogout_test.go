// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package login

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/loginutil"
)

func TestWriteLogoutSinkFromToken(t *testing.T) {
	newCmd := func(sink string) *cmd {
		c := New(cli.NewMockUi())
		c.tokenSinkFile = sink
		return c
	}

	t.Run("writes sidecar with owner-only perms when IDPLogoutURL is present", func(t *testing.T) {
		sink := filepath.Join(t.TempDir(), "token")
		c := newCmd(sink)

		url := "https://idp.example.com/logout?id_token_hint=xyz"
		c.writeLogoutSinkFromToken(&api.ACLToken{IDPLogoutURL: url})

		sidecar := sink + loginutil.IDPLogoutSuffix
		data, err := os.ReadFile(sidecar)
		require.NoError(t, err)
		require.Equal(t, url, string(data))

		info, err := os.Stat(sidecar)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("removes stale sidecar when IDPLogoutURL is empty", func(t *testing.T) {
		sink := filepath.Join(t.TempDir(), "token")
		sidecar := sink + loginutil.IDPLogoutSuffix
		require.NoError(t, os.WriteFile(sidecar, []byte("https://old.example.com/logout"), 0o600))

		c := newCmd(sink)
		c.writeLogoutSinkFromToken(&api.ACLToken{IDPLogoutURL: ""})

		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err), "stale sidecar should have been removed")
	})

	t.Run("no-op when no token sink file is configured", func(t *testing.T) {
		c := newCmd("")
		require.NotPanics(t, func() {
			c.writeLogoutSinkFromToken(&api.ACLToken{IDPLogoutURL: "https://idp.example.com/logout"})
		})
	})
}

// TestWriteLogoutSinkFromToken_WriteError verifies that a write error prints
// a warning and does not panic.
func TestWriteLogoutSinkFromToken_WriteError(t *testing.T) {
	// Use an existing regular file as the "parent directory" — MkdirAll fails on it.
	blockingFile := filepath.Join(t.TempDir(), "block")
	require.NoError(t, os.WriteFile(blockingFile, []byte("x"), 0o600))
	sink := filepath.Join(blockingFile, "token") // parent is a file, not a dir

	ui := cli.NewMockUi()
	c := New(ui)
	c.tokenSinkFile = sink

	// Should not panic; should print a warning.
	require.NotPanics(t, func() {
		c.writeLogoutSinkFromToken(&api.ACLToken{IDPLogoutURL: "https://idp.example.com/logout"})
	})
	require.Contains(t, ui.ErrorWriter.String(), "Error writing")
}
