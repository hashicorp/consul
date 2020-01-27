package rolecreate

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

func TestRoleCreateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleCreateCommand(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err)

	// create with policy by name
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=role-with-policy-by-name",
			"-description=test-role",
			"-policy-name=" + policy.Name,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
	}

	// create with policy by id
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=role-with-policy-by-id",
			"-description=test-role",
			"-policy-id=" + policy.ID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
	}

	// create with service identity
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=role-with-service-identity",
			"-description=test-role",
			"-service-identity=web",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
	}

	// create with service identity scoped to 2 DCs
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=role-with-service-identity-in-2-dcs",
			"-description=test-role",
			"-service-identity=db:abc,xyz",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
	}
}
