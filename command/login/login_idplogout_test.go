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

		sidecar := sink + idpLogoutSuffix
		data, err := os.ReadFile(sidecar)
		require.NoError(t, err)
		require.Equal(t, url, string(data))

		info, err := os.Stat(sidecar)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("removes stale sidecar when IDPLogoutURL is empty", func(t *testing.T) {
		sink := filepath.Join(t.TempDir(), "token")
		sidecar := sink + idpLogoutSuffix
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
