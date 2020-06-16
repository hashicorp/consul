package tokenupdate

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenUpdateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenUpdateCommand(t *testing.T) {
	t.Parallel()

	testDir := testutil.TempDir(t, "acl")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// create a token
	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// create a legacy token
	// nolint: staticcheck // we have to use the deprecated API to create a legacy token
	legacyTokenSecretID, _, err := client.ACL().Create(&api.ACLEntry{
		Name:  "Legacy token",
		Type:  "client",
		Rules: "service \"test\" { policy = \"write\" }",
	},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// We fetch the legacy token later to give server time to async background
	// upgrade it.

	run := func(t *testing.T, args []string) *api.ACLToken {
		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(append(args, "-format=json"))
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())

		var token api.ACLToken
		require.NoError(t, json.Unmarshal(ui.OutputWriter.Bytes(), &token))
		return &token
	}

	// update with node identity
	t.Run("node-identity", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-node-identity=foo:bar",
			"-description=test token",
		})

		require.Len(t, token.NodeIdentities, 1)
		require.Equal(t, "foo", token.NodeIdentities[0].NodeName)
		require.Equal(t, "bar", token.NodeIdentities[0].Datacenter)
	})

	t.Run("node-identity-merge", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-node-identity=bar:baz",
			"-description=test token",
			"-merge-node-identities",
		})

		require.Len(t, token.NodeIdentities, 2)
		expected := []*api.ACLNodeIdentity{
			&api.ACLNodeIdentity{
				NodeName:   "foo",
				Datacenter: "bar",
			},
			&api.ACLNodeIdentity{
				NodeName:   "bar",
				Datacenter: "baz",
			},
		}
		require.ElementsMatch(t, expected, token.NodeIdentities)
	})

	// update with policy by name
	t.Run("policy-name", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
		})

		require.Len(t, token.Policies, 1)
	})

	// update with policy by id
	t.Run("policy-id", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-policy-id=" + policy.ID,
			"-description=test token",
		})

		require.Len(t, token.Policies, 1)
	})

	// update with no description shouldn't delete the current description
	t.Run("merge-description", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
		})

		require.Equal(t, "test token", token.Description)
	})

	// Need legacy token now, hopefully server had time to generate an accessor ID
	// in the background but wait for it if not.
	var legacyToken *api.ACLToken
	retry.Run(t, func(r *retry.R) {
		// Fetch the legacy token via new API so we can use it's accessor ID
		legacyToken, _, err = client.ACL().TokenReadSelf(
			&api.QueryOptions{Token: legacyTokenSecretID})
		require.NoError(r, err)
		require.NotEmpty(r, legacyToken.AccessorID)
	})

	// upgrade legacy token should replace rules and leave token in a "new" state!
	t.Run("legacy-upgrade", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + legacyToken.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
			"-upgrade-legacy",
		})

		// Description shouldn't change
		require.Equal(t, "Legacy token", token.Description)
		require.Len(t, token.Policies, 1)
		// Rules should now be empty meaning this is no longer a legacy token
		require.Empty(t, token.Rules)
		// Secret should not have changes
		require.Equal(t, legacyToken.SecretID, token.SecretID)
	})
}

func TestTokenUpdateCommand_JSON(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	// Alias because we need to access require package in Retry below
	req := require.New(t)

	testDir := testutil.TempDir(t, "acl")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	req.NoError(err)

	// create a token
	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	req.NoError(err)

	t.Run("update with policy by name", func(t *testing.T) {
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
			"-format=json",
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())

		var jsonOutput json.RawMessage
		err := json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
		require.NoError(t, err, "token unmarshalling error")
	})
}
