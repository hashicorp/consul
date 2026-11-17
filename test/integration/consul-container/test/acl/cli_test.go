// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// TestACL_CLI drives the `consul acl` commands against a live agent. The
// commands authenticate with the bootstrap token through the environment.
func TestACL_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	admin := func(args string) string {
		return "CONSUL_HTTP_TOKEN=" + e.cluster.TokenBootstrap + " consul acl " + args
	}

	t.Run("commands without a token are denied", func(t *testing.T) {
		res := e.shell(t, "consul acl policy list")

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "Permission denied")
	})

	t.Run("a policy, role and token built from the command line are enforced", func(t *testing.T) {
		res := e.shell(t, admin(`policy create -name cli-policy -rules 'key_prefix "cli/" { policy = "read" }'`))
		require.Equal(t, 0, res.ExitCode, res.Output)

		res = e.shell(t, admin(`role create -name cli-role -policy-name cli-policy`))
		require.Equal(t, 0, res.ExitCode, res.Output)

		res = e.shell(t, admin(`token create -role-name cli-role -description cli-token -format=json 2>/dev/null`))
		require.Equal(t, 0, res.ExitCode, res.Output)
		var token api.ACLToken
		require.NoError(t, json.Unmarshal([]byte(res.Output), &token), res.Output)
		require.Equal(t, "cli-role", token.Roles[0].Name)

		_, err := e.adminClient.KV().Put(&api.KVPair{Key: "cli/value", Value: []byte("x")}, nil)
		require.NoError(t, err)
		_, err = e.adminClient.KV().Put(&api.KVPair{Key: "elsewhere/value", Value: []byte("x")}, nil)
		require.NoError(t, err)

		c := e.tokenClient(t, token.SecretID)
		pair, _, err := c.KV().Get("cli/value", nil)
		require.NoError(t, err)
		require.NotNil(t, pair)
		_, _, err = c.KV().Get("elsewhere/value", nil)
		requireDenied(t, err, "outside the policy")

		// The new token identifies itself through the CLI too.
		res = e.shell(t, "CONSUL_HTTP_TOKEN="+token.SecretID+" consul acl token read -self")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, token.AccessorID)
		require.Contains(t, res.Output, "cli-role")

		res = e.shell(t, admin("token delete -id "+token.AccessorID))
		require.Equal(t, 0, res.ExitCode, res.Output)
		eventually(t, func(r *retry.R) {
			_, _, err := c.KV().Get("cli/value", nil)
			require.Error(r, err)
		})
	})

	t.Run("listing", func(t *testing.T) {
		res := e.shell(t, admin("policy list"))
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "cli-policy")
		require.Contains(t, res.Output, "global-management")

		res = e.shell(t, admin("role list"))
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "cli-role")

		res = e.shell(t, admin("templated-policy list"))
		require.Equal(t, 0, res.ExitCode, res.Output)
		for _, name := range []string{"builtin/service", "builtin/node", "builtin/dns", "builtin/api-gateway"} {
			require.Contains(t, res.Output, name)
		}
	})

	t.Run("a token can be created from a template", func(t *testing.T) {
		res := e.shell(t, admin(`token create -templated-policy builtin/service -var name:cli-svc -format=json 2>/dev/null`))
		require.Equal(t, 0, res.ExitCode, res.Output)
		var token api.ACLToken
		require.NoError(t, json.Unmarshal([]byte(res.Output), &token), res.Output)

		c := e.tokenClient(t, token.SecretID)
		require.NoError(t, registerService(c, "cli-svc"))
		requireDenied(t, registerService(c, "cli-other"), "outside the template")
	})

	t.Run("previewing a template", func(t *testing.T) {
		res := e.shell(t, admin("templated-policy preview -name builtin/service -var name:previewed"))

		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, `service "previewed"`)
		require.True(t, strings.Contains(res.Output, `"previewed-sidecar-proxy"`))
	})
}
