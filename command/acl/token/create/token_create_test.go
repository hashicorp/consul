package tokencreate

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
	"github.com/stretchr/testify/require"
)

func TestTokenCreateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenCreateCommand(t *testing.T) {
	t.Parallel()
	require := require.New(t)

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

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(err)

	// create with policy by name
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
		}

		code := cmd.Run(args)
		require.Equal(code, 0)
		require.Empty(ui.ErrorWriter.String())
	}

	// create with policy by id
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-id=" + policy.ID,
			"-description=test token",
		}

		code := cmd.Run(args)
		require.Empty(ui.ErrorWriter.String())
		require.Equal(code, 0)
	}

	// create with accessor and secret
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-id=" + policy.ID,
			"-description=test token",
			"-accessor=3d852bb8-5153-4388-a3ca-8ca78661889f",
			"-secret=3a69a8d8-c4d4-485d-9b19-b5b61648ea0c",
		}

		code := cmd.Run(args)
		require.Empty(ui.ErrorWriter.String())
		require.Equal(code, 0)

		conf := api.DefaultConfig()
		conf.Address = a.HTTPAddr()
		conf.Token = "root"

		// going to use the API client to grab the token - we could potentially try to grab the values
		// out of the command output but this seems easier.
		client, err := api.NewClient(conf)
		require.NoError(err)
		require.NotNil(client)

		token, _, err := client.ACL().TokenRead("3d852bb8-5153-4388-a3ca-8ca78661889f", nil)
		require.NoError(err)
		require.Equal("3d852bb8-5153-4388-a3ca-8ca78661889f", token.AccessorID)
		require.Equal("3a69a8d8-c4d4-485d-9b19-b5b61648ea0c", token.SecretID)
	}
}
