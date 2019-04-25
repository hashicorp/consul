package tokenupdate

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestTokenUpdateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenUpdateCommand(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	// Alias because we need to access require package in Retry below
	req := require.New(t)

	testDir := testutil.TempDir(t, "acl")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, t.Name(), `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	a.Agent.LogWriter = logger.NewLogWriter(512)

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

	// create a legacy token
	legacyTokenSecretID, _, err := client.ACL().Create(&api.ACLEntry{
		Name:  "Legacy token",
		Type:  "client",
		Rules: "service \"test\" { policy = \"write\" }",
	},
		&api.WriteOptions{Token: "root"},
	)
	req.NoError(err)

	// We fetch the legacy token later to give server time to async background
	// upgrade it.

	// update with policy by name
	{
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())

		token, _, err := client.ACL().TokenRead(
			token.AccessorID,
			&api.QueryOptions{Token: "root"},
		)
		assert.NoError(err)
		assert.NotNil(token)
	}

	// update with policy by id
	{
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-policy-id=" + policy.ID,
			"-description=test token",
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())

		token, _, err := client.ACL().TokenRead(
			token.AccessorID,
			&api.QueryOptions{Token: "root"},
		)
		assert.NoError(err)
		assert.NotNil(token)
	}

	// update with no description shouldn't delete the current description
	{
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())

		token, _, err := client.ACL().TokenRead(
			token.AccessorID,
			&api.QueryOptions{Token: "root"},
		)
		assert.NoError(err)
		assert.NotNil(token)
		assert.Equal("test token", token.Description)
	}

	// Need legacy token now, hopefully server had time to generate an accessor ID
	// in the background but wait for it if not.
	var legacyToken *api.ACLToken
	retry.Run(t, func(r *retry.R) {
		// Fetch the legacy token via new API so we can use it's accessor ID
		legacyToken, _, err = client.ACL().TokenReadSelf(
			&api.QueryOptions{Token: legacyTokenSecretID})
		r.Check(err)
		require.NotEmpty(r, legacyToken.AccessorID)
	})

	// upgrade legacy token should replace rules and leave token in a "new" state!
	{
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + legacyToken.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
			"-upgrade-legacy",
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())

		gotToken, _, err := client.ACL().TokenRead(
			legacyToken.AccessorID,
			&api.QueryOptions{Token: "root"},
		)
		assert.NoError(err)
		assert.NotNil(gotToken)
		// Description shouldn't change
		assert.Equal("Legacy token", gotToken.Description)
		assert.Len(gotToken.Policies, 1)
		// Rules should now be empty meaning this is no longer a legacy token
		assert.Empty(gotToken.Rules)
		// Secret should not have changes
		assert.Equal(legacyToken.SecretID, gotToken.SecretID)
	}
}
