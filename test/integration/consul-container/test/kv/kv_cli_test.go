// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKV_CLI drives the `consul kv` commands against a live agent.
func TestKV_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})

	t.Run("put and get", func(t *testing.T) {
		res := e.shell(t, "consul kv put cli/one 1 && consul kv put -flags=7 cli/two 2")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "Success! Data written to: cli/one")

		res = e.shell(t, "consul kv get cli/one")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Equal(t, "1", strings.TrimSpace(res.Output))

		res = e.shell(t, "consul kv get -detailed cli/two")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Regexp(t, `(?m)^Flags\s+7$`, res.Output)
		require.Regexp(t, `(?m)^Key\s+cli/two$`, res.Output)
		require.Regexp(t, `(?m)^Value\s+2$`, res.Output)

		pair := mustGet(t, e.client.KV(), "cli/two")
		require.EqualValues(t, 7, pair.Flags, "the CLI and API see the same data")
	})

	t.Run("values can be read from stdin", func(t *testing.T) {
		res := e.shell(t, "printf 'from stdin' | consul kv put cli/stdin - && consul kv get cli/stdin")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "from stdin")
	})

	t.Run("listing", func(t *testing.T) {
		res := e.shell(t, "consul kv put cli/nested/x 3 >/dev/null && consul kv get -recurse cli/")
		require.Equal(t, 0, res.ExitCode, res.Output)
		for _, want := range []string{"cli/one:1", "cli/two:2", "cli/nested/x:3"} {
			require.Contains(t, res.Output, want)
		}

		res = e.shell(t, "consul kv get -keys -separator=/ cli/")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Equal(t, []string{"cli/nested/", "cli/one", "cli/stdin", "cli/two"}, strings.Fields(res.Output))
	})

	t.Run("a missing key is an error", func(t *testing.T) {
		res := e.shell(t, "consul kv get cli/missing")

		require.Equal(t, 1, res.ExitCode)
		require.Contains(t, res.Output, "No key exists at: cli/missing")
	})

	t.Run("check-and-set", func(t *testing.T) {
		res := e.shell(t, "consul kv put -cas -modify-index=0 cli/one overwritten")
		require.Equal(t, 1, res.ExitCode)
		require.Contains(t, res.Output, "CAS performed with index=0 and key already exists")

		res = e.shell(t, "consul kv put -cas -modify-index=1 cli/one overwritten")
		require.Equal(t, 1, res.ExitCode)
		require.Contains(t, res.Output, "CAS failed")

		res = e.shell(t, "consul kv put -cas -modify-index=0 cli/brand-new created")
		require.Equal(t, 0, res.ExitCode, res.Output)

		res = e.shell(t, "consul kv delete -cas -modify-index=1 cli/two")
		require.Equal(t, 1, res.ExitCode)
		require.Contains(t, res.Output, "CAS failed")
		mustGet(t, e.client.KV(), "cli/two")
	})

	t.Run("export and import round trip", func(t *testing.T) {
		res := e.shell(t, "consul kv export cli/ > /tmp/kv-export.json && cat /tmp/kv-export.json")
		require.Equal(t, 0, res.ExitCode, res.Output)

		var exported []struct {
			Key   string `json:"key"`
			Flags uint64 `json:"flags"`
			Value string `json:"value"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.Output), &exported))
		var keys []string
		for _, item := range exported {
			keys = append(keys, item.Key)
		}
		require.ElementsMatch(t, []string{"cli/brand-new", "cli/nested/x", "cli/one", "cli/stdin", "cli/two"}, keys)

		res = e.shell(t, "consul kv delete -recurse cli/")
		require.Equal(t, 0, res.ExitCode, res.Output)
		list, _, err := e.client.KV().List("cli/", nil)
		require.NoError(t, err)
		require.Empty(t, list)

		res = e.shell(t, "consul kv import @/tmp/kv-export.json")
		require.Equal(t, 0, res.ExitCode, res.Output)

		restored := mustGet(t, e.client.KV(), "cli/two")
		require.Equal(t, "2", string(restored.Value))
		require.EqualValues(t, 7, restored.Flags, "flags survive an export/import")
		list, _, err = e.client.KV().List("cli/", nil)
		require.NoError(t, err)
		require.Len(t, list, 5)
	})

	t.Run("delete", func(t *testing.T) {
		res := e.shell(t, "consul kv delete cli/one")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "Success! Deleted key: cli/one")

		res = e.shell(t, "consul kv delete -recurse cli/")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "Success! Deleted keys with prefix: cli/")
	})
}
