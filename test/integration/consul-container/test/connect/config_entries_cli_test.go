// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

// TestConfigEntries_CLI drives the `consul config` commands against a live
// agent: write from an HCL file, read, list, and delete.
func TestConfigEntries_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	write := func(t *testing.T, path, hcl string) {
		t.Helper()
		res := e.shell(t, "cat > "+path+" <<'EOF'\n"+hcl+"\nEOF\nconsul config write "+path)
		require.Equal(t, 0, res.ExitCode, res.Output)
	}

	t.Run("write creates the entry", func(t *testing.T) {
		write(t, "/tmp/cli-defaults.hcl", `
Kind     = "service-defaults"
Name     = "cli-web"
Protocol = "http"
Meta {
  owner = "platform"
}`)

		entry, _, err := e.server.ConfigEntries().Get(api.ServiceDefaults, "cli-web", nil)
		require.NoError(t, err)
		sd := entry.(*api.ServiceConfigEntry)
		require.Equal(t, "http", sd.Protocol)
		require.Equal(t, "platform", sd.Meta["owner"])
	})

	t.Run("write updates an existing entry", func(t *testing.T) {
		write(t, "/tmp/cli-defaults.hcl", `
Kind     = "service-defaults"
Name     = "cli-web"
Protocol = "grpc"`)

		entry, _, err := e.server.ConfigEntries().Get(api.ServiceDefaults, "cli-web", nil)
		require.NoError(t, err)
		require.Equal(t, "grpc", entry.(*api.ServiceConfigEntry).Protocol)
	})

	t.Run("read returns the entry as JSON", func(t *testing.T) {
		res := e.cliStdout(t, "config", "read", "-kind", "service-defaults", "-name", "cli-web")
		require.Equal(t, 0, res.ExitCode, res.Output)

		got := decodeJSON(t, res.Output)
		require.Equal(t, "service-defaults", got["Kind"])
		require.Equal(t, "cli-web", got["Name"])
		require.Equal(t, "grpc", got["Protocol"])
	})

	t.Run("list shows entries of a kind", func(t *testing.T) {
		write(t, "/tmp/cli-defaults-2.hcl", `
Kind     = "service-defaults"
Name     = "cli-api"
Protocol = "tcp"`)

		res := e.cliStdout(t, "config", "list", "-kind", "service-defaults")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "cli-web")
		require.Contains(t, res.Output, "cli-api")
	})

	t.Run("invalid entries are rejected with a non-zero exit code", func(t *testing.T) {
		res := e.shell(t, "cat > /tmp/cli-bad.hcl <<'EOF'\nKind = \"service-defaults\"\nName = \"cli-bad\"\nProtocol = \"carrier-pigeon\"\nEOF\nconsul config write /tmp/cli-bad.hcl")
		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "protocol")
	})

	t.Run("delete removes the entry", func(t *testing.T) {
		res := e.cliCombined(t, "config", "delete", "-kind", "service-defaults", "-name", "cli-web")
		require.Equal(t, 0, res.ExitCode, res.Output)

		_, _, err := e.server.ConfigEntries().Get(api.ServiceDefaults, "cli-web", nil)
		require.Error(t, err)

		res = e.cliCombined(t, "config", "read", "-kind", "service-defaults", "-name", "cli-web")
		require.NotEqual(t, 0, res.ExitCode)
	})
}
