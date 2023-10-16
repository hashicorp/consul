// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package write

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/config"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestConfigWrite_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestConfigWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	t.Run("File", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		f := testutil.TempFile(t, "config-write-svc-web.hcl")
		_, err := f.WriteString(`
      Kind = "service-defaults"
      Name = "web"
      Protocol = "udp"
      `)

		require.NoError(t, err)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			f.Name(),
		}

		code := c.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(),
			`Config entry written: service-defaults/web`)
		require.Equal(t, 0, code)

		entry, _, err := client.ConfigEntries().Get("service-defaults", "web", nil)
		require.NoError(t, err)
		svc, ok := entry.(*api.ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, api.ServiceDefaults, svc.Kind)
		require.Equal(t, "web", svc.Name)
		require.Equal(t, "udp", svc.Protocol)
	})

	t.Run("Stdin", func(t *testing.T) {
		stdinR, stdinW := io.Pipe()

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdinR

		go func() {
			stdinW.Write([]byte(`{
            "Kind": "proxy-defaults",
            "Name": "global",
            "Config": {
               "foo": "bar",
               "bar": 1.0
            }
         }`))
			stdinW.Close()
		}()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-",
		}

		code := c.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(),
			`Config entry written: proxy-defaults/global`)
		require.Equal(t, 0, code)

		entry, _, err := client.ConfigEntries().Get(api.ProxyDefaults, api.ProxyConfigGlobal, nil)
		require.NoError(t, err)
		proxy, ok := entry.(*api.ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, api.ProxyDefaults, proxy.Kind)
		require.Equal(t, api.ProxyConfigGlobal, proxy.Name)
		require.Equal(t, map[string]interface{}{"foo": "bar", "bar": 1.0}, proxy.Config)
	})

	t.Run("No config", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		code := c.Run([]string{})
		require.NotEqual(t, 0, code)
		require.NotEmpty(t, ui.ErrorWriter.String())
	})

	t.Run("mesh config entry", func(t *testing.T) {
		stdin := new(bytes.Buffer)
		stdin.WriteString(`
kind = "mesh"
meta {
	"foo" = "bar"
	"gir" = "zim"
}
transparent_proxy {
	mesh_destinations_only = true
}
http {
    sanitize_x_forwarded_client_cert = true
}
`)

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdin

		code := c.Run([]string{"-http-addr=" + a.HTTPAddr(), "-"})
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(),
			`Config entry written: mesh/mesh`)
		require.Equal(t, 0, code)

		entry, _, err := client.ConfigEntries().Get(api.MeshConfig, api.MeshConfigMesh, nil)
		require.NoError(t, err)
		proxy, ok := entry.(*api.MeshConfigEntry)
		require.True(t, ok)
		require.Equal(t, map[string]string{"foo": "bar", "gir": "zim"}, proxy.Meta)
		require.True(t, proxy.TransparentProxy.MeshDestinationsOnly)

		require.True(t, proxy.HTTP.SanitizeXForwardedClientCert)
	})

	// Test that if name isn't set (which isn't required for proxy-defaults because the name defaults to
	// "global"), the CLI response still says "config entry written proxy-defaults/global".
	t.Run("proxy defaults config entry without name set", func(t *testing.T) {
		stdin := new(bytes.Buffer)
		stdin.WriteString(`
kind = "proxy-defaults"
`)

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdin

		code := c.Run([]string{"-http-addr=" + a.HTTPAddr(), "-"})
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(),
			`Config entry written: proxy-defaults/global`)
		require.Equal(t, 0, code)
	})
}

func TestConfigWrite_Warning(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	cases := map[string]struct {
		entry   string
		warning string
	}{
		"service-defaults no warning": {
			entry: `
				Kind = "service-defaults"
				Name = "web"
				MutualTLSMode = "strict"
			`,
		},
		"proxy-defaults no warning": {
			entry: `
				Kind = "proxy-defaults"
				Name = "global"
				MutualTLSMode = "strict"
			`,
		},
		"mesh config entry no warning": {
			entry: `
				Kind = "mesh"
				AllowEnablingPermissiveMutualTLS = false
			`,
		},
		"service-defaults warning on MutualTLSMode=permissive": {
			entry: `
				Kind = "service-defaults"
				Name = "web"
				MutualTLSMode = "permissive"
			`,
			warning: config.WarningServiceDefaultsPermissiveMTLS,
		},
		"proxy-defaults warning on MutualTLSMode=permissive": {
			entry: `
				Kind = "proxy-defaults"
				Name = "global"
				MutualTLSMode = "permissive"
			`,
			warning: config.WarningProxyDefaultsPermissiveMTLS,
		},
		"mesh config entry warning on AllowEnablingPermissiveMutualTLS=true": {
			entry: `
				Kind = "mesh"
				AllowEnablingPermissiveMutualTLS = true
			`,
			warning: config.WarningMeshAllowEnablingPermissiveMutualTLS,
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			// Always reset this setting to avoid causing validation errors.
			_, _, err := client.ConfigEntries().Set(&api.MeshConfigEntry{
				AllowEnablingPermissiveMutualTLS: true,
			}, nil)
			require.NoError(t, err)

			f := testutil.TempFile(t, "config-write-warning-*.hcl")
			_, err = f.WriteString(c.entry)
			require.NoError(t, err)

			ui := cli.NewMockUi()
			code := New(ui).Run([]string{
				"-http-addr=" + a.HTTPAddr(),
				f.Name(),
			})

			require.Equal(t, 0, code)
			require.Contains(t, ui.OutputWriter.String(), `Config entry written`)

			errMsg := ui.ErrorWriter.String()
			if c.warning != "" {
				require.Contains(t, errMsg, c.warning)
			} else {
				require.Empty(t, errMsg)
			}
		})
	}
}

func requireContainsLower(t *testing.T, haystack, needle string) {
	t.Helper()
	require.Contains(t, strings.ToLower(haystack), strings.ToLower(needle))
}
