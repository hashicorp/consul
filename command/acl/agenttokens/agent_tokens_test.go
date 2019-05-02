package agenttokens

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestAgentTokensCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestAgentTokensCommand(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

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
	cmd := New(ui)

	// Create a token to set
	client := a.Client()

	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(err)

	// default token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"default",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())
	}

	// agent token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"agent",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())
	}

	// master token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"master",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())
	}

	// replication token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"replication",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())
	}
}
