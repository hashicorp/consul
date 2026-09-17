// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package logout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/command/loginutil"
)

func TestMaybeIDPLogout(t *testing.T) {
	t.Run("prints URL and removes sidecar when present", func(t *testing.T) {
		tokenFile := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(tokenFile, []byte("secret"), 0o600))
		sidecar := tokenFile + loginutil.IDPLogoutSuffix
		// Include surrounding whitespace to verify it is trimmed.
		require.NoError(t, os.WriteFile(sidecar, []byte("  https://idp.example.com/logout?id_token_hint=xyz  \n"), 0o600))

		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.http.SetTokenFile(tokenFile))
		c.maybeIDPLogout()

		// The URL must be printed (not opened in a browser).
		require.Contains(t, ui.OutputWriter.String(), "https://idp.example.com/logout?id_token_hint=xyz")
		require.Contains(t, ui.OutputWriter.String(), "open the following URL")
		// The sidecar must be removed after logout.
		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err), "sidecar should be removed after logout")
	})

	t.Run("no-op when sidecar is absent", func(t *testing.T) {
		tokenFile := filepath.Join(t.TempDir(), "token")

		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.http.SetTokenFile(tokenFile))
		c.maybeIDPLogout()

		require.Empty(t, ui.OutputWriter.String(), "nothing should be printed without a sidecar")
	})

	t.Run("no-op when no token file is configured", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		c.maybeIDPLogout()

		require.Empty(t, ui.OutputWriter.String(), "nothing should be printed without a token file")
	})
}

// TestMaybeIDPLogout_EmptySidecar verifies that an empty sidecar file is a no-op.
func TestMaybeIDPLogout_EmptySidecar(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	sidecar := tokenFile + loginutil.IDPLogoutSuffix
	require.NoError(t, os.WriteFile(sidecar, []byte("   \n"), 0o600))

	ui := cli.NewMockUi()
	c := New(ui)
	require.NoError(t, c.http.SetTokenFile(tokenFile))
	c.maybeIDPLogout()

	require.Empty(t, ui.OutputWriter.String(), "nothing should be printed for a whitespace-only sidecar")
	// Sidecar is still cleaned up.
	_, err := os.Stat(sidecar)
	require.True(t, os.IsNotExist(err), "sidecar should be removed")
}

// TestMaybeIDPLogout_RejectsNonHTTP verifies that a tampered sidecar containing
// a non-http(s) URL is not printed (defense against terminal-escape or other
// unexpected content), while the sidecar is still cleaned up.
func TestMaybeIDPLogout_RejectsNonHTTP(t *testing.T) {
	for _, bad := range []string{
		"file:///etc/passwd",
		"ftp://example.com/x",
		"javascript:alert(1)",
		"://missing-scheme",
	} {
		tokenFile := filepath.Join(t.TempDir(), "token")
		sidecar := tokenFile + loginutil.IDPLogoutSuffix
		require.NoError(t, os.WriteFile(sidecar, []byte(bad), 0o600))

		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.http.SetTokenFile(tokenFile))
		c.maybeIDPLogout()

		require.Empty(t, ui.OutputWriter.String(), "must not print non-http(s) URL %q", bad)
		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err), "sidecar should be removed even for rejected URL %q", bad)
	}
}
