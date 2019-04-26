package roledelete

import (
	"fmt"
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

func TestRoleDeleteCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleDeleteCommand(t *testing.T) {
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

	client := a.Client()

	t.Run("id or name required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Must specify the -id or -name parameters")
	})

	t.Run("delete works", func(t *testing.T) {
		// Create a role
		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name: "test-role-for-id-delete",
				ServiceIdentities: []*api.ACLServiceIdentity{
					&api.ACLServiceIdentity{
						ServiceName: "fake",
					},
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id=" + role.ID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("deleted successfully"))
		require.Contains(t, output, role.ID)

		role, _, err = client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.Nil(t, role)
	})

	t.Run("delete works via prefixes", func(t *testing.T) {
		// Create a role
		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name: "test-role-for-id-prefix-delete",
				ServiceIdentities: []*api.ACLServiceIdentity{
					&api.ACLServiceIdentity{
						ServiceName: "fake",
					},
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id=" + role.ID[0:5],
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("deleted successfully"))
		require.Contains(t, output, role.ID)

		role, _, err = client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.Nil(t, role)
	})
}
