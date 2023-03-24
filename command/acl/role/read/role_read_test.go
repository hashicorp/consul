package roleread

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleReadCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			initial_management = "root"
		}
	}`)

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
		require.Contains(t, ui.ErrorWriter.String(), "Must specify either the -id or -name parameters")
	})

	t.Run("read by id not found", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id=" + fakeID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Role not found with ID")
	})

	t.Run("read by name not found", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=blah",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Role not found with name")
	})

	t.Run("read by id", func(t *testing.T) {
		// create a role
		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name: "test-role-by-id",
				ServiceIdentities: []*api.ACLServiceIdentity{
					{
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
		require.Contains(t, output, fmt.Sprintf("test-role"))
		require.Contains(t, output, role.ID)
	})

	t.Run("read by id prefix", func(t *testing.T) {
		// create a role
		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name: "test-role-by-id-prefix",
				ServiceIdentities: []*api.ACLServiceIdentity{
					{
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
		require.Contains(t, output, fmt.Sprintf("test-role"))
		require.Contains(t, output, role.ID)
	})

	t.Run("read by name", func(t *testing.T) {
		// create a role
		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name: "test-role-by-name",
				ServiceIdentities: []*api.ACLServiceIdentity{
					{
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
			"-name=" + role.Name,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("test-role"))
		require.Contains(t, output, role.ID)
	})
}

func TestRoleReadCommand_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			initial_management = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client := a.Client()

	t.Run("read by id", func(t *testing.T) {
		// create a role
		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name: "test-role-by-id",
				ServiceIdentities: []*api.ACLServiceIdentity{
					{
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
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("test-role"))
		require.Contains(t, output, role.ID)

		var jsonOutput json.RawMessage
		err = json.Unmarshal([]byte(output), &jsonOutput)
		assert.NoError(t, err)
	})
}
