// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package logout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestMaybeIDPLogout(t *testing.T) {
	// Stub the browser opener so tests never actually launch a browser.
	stubOpener := func(captured *string) func() {
		orig := openURL
		openURL = func(u string) error {
			if captured != nil {
				*captured = u
			}
			return nil
		}
		return func() { openURL = orig }
	}

	t.Run("opens URL and removes sidecar when present", func(t *testing.T) {
		tokenFile := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(tokenFile, []byte("secret"), 0o600))
		sidecar := tokenFile + idpLogoutSuffix
		// Include surrounding whitespace to verify it is trimmed.
		require.NoError(t, os.WriteFile(sidecar, []byte("  https://idp.example.com/logout?id_token_hint=xyz  \n"), 0o600))

		var opened string
		defer stubOpener(&opened)()

		c := New(cli.NewMockUi())
		require.NoError(t, c.http.SetTokenFile(tokenFile))
		c.maybeIDPLogout()

		require.Equal(t, "https://idp.example.com/logout?id_token_hint=xyz", opened)
		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err), "sidecar should be removed after logout")
	})

	t.Run("no-op when sidecar is absent", func(t *testing.T) {
		tokenFile := filepath.Join(t.TempDir(), "token")

		called := false
		orig := openURL
		openURL = func(string) error { called = true; return nil }
		defer func() { openURL = orig }()

		c := New(cli.NewMockUi())
		require.NoError(t, c.http.SetTokenFile(tokenFile))
		c.maybeIDPLogout()

		require.False(t, called, "browser must not open without a sidecar")
	})

	t.Run("no-op when no token file is configured", func(t *testing.T) {
		called := false
		orig := openURL
		openURL = func(string) error { called = true; return nil }
		defer func() { openURL = orig }()

		c := New(cli.NewMockUi())
		c.maybeIDPLogout()

		require.False(t, called, "browser must not open without a token file")
	})
}

// TestOpenURL_RejectsNonHTTP guards the injection defense: the real opener must
// refuse anything that is not a well-formed http(s) URL, so a tampered sidecar
// cannot smuggle shell/command metacharacters to the OS.
func TestOpenURL_RejectsNonHTTP(t *testing.T) {
	for _, bad := range []string{
		"file:///etc/passwd",
		"ftp://example.com/x",
		"javascript:alert(1)",
		"://missing-scheme",
	} {
		require.Error(t, openURL(bad), "expected rejection of %q", bad)
	}
}
