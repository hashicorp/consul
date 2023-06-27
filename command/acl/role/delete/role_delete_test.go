// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package roledelete

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestRoleDeleteCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleDeleteCommand(t *testing.T) {
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
		require.Contains(t, ui.ErrorWriter.String(), "Must specify the -id or -name parameters")
	})

	t.Run("delete works", func(t *testing.T) {
		// Create a role
		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name: "test-role-for-id-delete",
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
