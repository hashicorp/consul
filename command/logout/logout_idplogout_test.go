// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package logout

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
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
		t.Setenv(api.HTTPTokenFileEnvName, "")
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

// TestMaybeIDPLogout_OpenBrowser verifies the -open-browser behavior: the URL
// is always printed, the browser is launched only when the flag is set, and a
// browser launch failure is reported as a warning without failing logout.
func TestMaybeIDPLogout_OpenBrowser(t *testing.T) {
	const logoutURL = "https://idp.example.com/api/oidc/logout?id_token_hint=xyz"

	setup := func(t *testing.T, args ...string) (*cmd, *cli.MockUi, *[]string, string) {
		tokenFile := filepath.Join(t.TempDir(), "token")
		sidecar := tokenFile + loginutil.IDPLogoutSuffix
		require.NoError(t, os.WriteFile(sidecar, []byte(logoutURL), 0o600))

		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.flags.Parse(args))
		require.NoError(t, c.http.SetTokenFile(tokenFile))

		var opened []string
		c.openURL = func(u string) error {
			opened = append(opened, u)
			return nil
		}
		return c, ui, &opened, sidecar
	}

	t.Run("does not open browser by default", func(t *testing.T) {
		c, ui, opened, sidecar := setup(t)
		c.maybeIDPLogout()

		require.Empty(t, *opened, "browser must not be opened without -open-browser")
		require.Contains(t, ui.OutputWriter.String(), logoutURL)
		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("opens browser and prints URL with -open-browser", func(t *testing.T) {
		c, ui, opened, sidecar := setup(t, "-open-browser")
		c.maybeIDPLogout()

		require.Equal(t, []string{logoutURL}, *opened)
		require.Contains(t, ui.OutputWriter.String(), logoutURL)
		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("warns when browser cannot be opened", func(t *testing.T) {
		c, ui, _, sidecar := setup(t, "-open-browser")
		c.openURL = func(string) error { return errors.New("no browser available") }
		c.maybeIDPLogout()

		require.Contains(t, ui.OutputWriter.String(), logoutURL)
		require.Contains(t, ui.ErrorWriter.String(), "no browser available")
		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("does not open browser without a sidecar", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.flags.Parse([]string{"-open-browser"}))
		require.NoError(t, c.http.SetTokenFile(filepath.Join(t.TempDir(), "token")))
		c.openURL = func(string) error {
			t.Fatal("browser must not be opened without an IdP logout URL")
			return nil
		}
		c.maybeIDPLogout()
		require.Empty(t, ui.OutputWriter.String())
	})
}

// TestApplyOpenBrowserEnv verifies that CONSUL_LOGOUT_OPEN_BROWSER sets the
// default for -open-browser, that an explicit flag overrides it, and that an
// invalid value is ignored with a warning.
func TestApplyOpenBrowserEnv(t *testing.T) {
	cases := map[string]struct {
		env      string
		args     []string
		expected bool
		warn     bool
	}{
		"unset defaults to false":           {env: "", expected: false},
		"true enables":                      {env: "true", expected: true},
		"1 enables":                         {env: "1", expected: true},
		"false disables":                    {env: "false", expected: false},
		"flag false overrides env true":     {env: "true", args: []string{"-open-browser=false"}, expected: false},
		"flag true overrides env false":     {env: "false", args: []string{"-open-browser"}, expected: true},
		"invalid value is ignored, warns":   {env: "yes-please", expected: false, warn: true},
		"surrounding whitespace is trimmed": {env: "  true \n", expected: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envOpenBrowser, tc.env)

			ui := cli.NewMockUi()
			c := New(ui)
			c.applyOpenBrowserEnv()
			require.NoError(t, c.flags.Parse(tc.args))

			require.Equal(t, tc.expected, c.openBrowser)
			if tc.warn {
				require.Contains(t, ui.ErrorWriter.String(), envOpenBrowser)
			} else {
				require.Empty(t, ui.ErrorWriter.String())
			}
		})
	}
}

// TestMaybeIDPLogout_TokenFileSource verifies the sidecar is resolved from the
// same token source the API client uses, including CONSUL_HTTP_TOKEN_FILE,
// with the same precedence (-token-file > -token > CONSUL_HTTP_TOKEN_FILE).
func TestMaybeIDPLogout_TokenFileSource(t *testing.T) {
	const logoutURL = "https://idp.example.com/api/oidc/logout?id_token_hint=xyz"

	writeSidecar := func(t *testing.T) (string, string) {
		tokenFile := filepath.Join(t.TempDir(), "token")
		sidecar := tokenFile + loginutil.IDPLogoutSuffix
		require.NoError(t, os.WriteFile(sidecar, []byte(logoutURL), 0o600))
		return tokenFile, sidecar
	}

	t.Run("uses CONSUL_HTTP_TOKEN_FILE when no flags are set", func(t *testing.T) {
		tokenFile, sidecar := writeSidecar(t)
		t.Setenv(api.HTTPTokenFileEnvName, tokenFile)

		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.flags.Parse(nil))
		c.maybeIDPLogout()

		require.Contains(t, ui.OutputWriter.String(), logoutURL)
		_, err := os.Stat(sidecar)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("-token-file flag takes precedence over CONSUL_HTTP_TOKEN_FILE", func(t *testing.T) {
		flagFile, flagSidecar := writeSidecar(t)
		envFile, envSidecar := writeSidecar(t)
		t.Setenv(api.HTTPTokenFileEnvName, envFile)

		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.flags.Parse([]string{"-token-file", flagFile}))
		c.maybeIDPLogout()

		_, err := os.Stat(flagSidecar)
		require.True(t, os.IsNotExist(err), "flag sidecar should be consumed")
		_, err = os.Stat(envSidecar)
		require.NoError(t, err, "env sidecar must be left untouched")
	})

	t.Run("-token flag means the token was not read from CONSUL_HTTP_TOKEN_FILE", func(t *testing.T) {
		envFile, envSidecar := writeSidecar(t)
		t.Setenv(api.HTTPTokenFileEnvName, envFile)

		ui := cli.NewMockUi()
		c := New(ui)
		require.NoError(t, c.flags.Parse([]string{"-token", "some-token"}))
		c.maybeIDPLogout()

		require.Empty(t, ui.OutputWriter.String())
		_, err := os.Stat(envSidecar)
		require.NoError(t, err, "env sidecar must be left untouched")
	})
}
