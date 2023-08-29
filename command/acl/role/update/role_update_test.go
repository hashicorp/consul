// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package roleupdate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	uuid "github.com/hashicorp/go-uuid"
)

func TestRoleUpdateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleUpdateCommand(t *testing.T) {
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

	// Create 2 policies
	policy1, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy1"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)
	policy2, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy2"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// create a role
	role, _, err := client.ACL().RoleCreate(
		&api.ACLRole{
			Name: "test-role",
			ServiceIdentities: []*api.ACLServiceIdentity{
				{
					ServiceName: "fake",
				},
			},
		},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	run := func(t *testing.T, args []string) *api.ACLRole {
		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(append(args, "-format=json", "-http-addr="+a.HTTPAddr()))
		require.Equal(t, 0, code, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		var role api.ACLRole
		require.NoError(t, json.Unmarshal(ui.OutputWriter.Bytes(), &role))
		return &role
	}

	t.Run("update a role that does not exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + fakeID,
			"-token=root",
			"-policy-name=" + policy1.Name,
			"-description=test role edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Role not found with ID")
	})

	t.Run("update with policy by name", func(t *testing.T) {
		_ = run(t, []string{
			"-id=" + role.ID,
			"-token=root",
			"-policy-name=" + policy1.Name,
			"-description=test role edited",
		})

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 1)
		require.Len(t, role.ServiceIdentities, 1)
	})

	t.Run("update with policy by id", func(t *testing.T) {
		// also update with no description shouldn't delete the current
		// description
		_ = run(t, []string{
			"-id=" + role.ID,
			"-token=root",
			"-policy-id=" + policy2.ID,
		})

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 2)
		require.Len(t, role.ServiceIdentities, 1)
	})

	t.Run("update with service identity", func(t *testing.T) {
		_ = run(t, []string{
			"-id=" + role.ID,
			"-token=root",
			"-service-identity=web",
		})

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 2)
		require.Len(t, role.ServiceIdentities, 2)
	})

	t.Run("update with service identity scoped to 2 DCs", func(t *testing.T) {
		_ = run(t, []string{
			"-id=" + role.ID,
			"-token=root",
			"-service-identity=db:abc,xyz",
		})

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 2)
		require.Len(t, role.ServiceIdentities, 3)
	})

	t.Run("update with node identity", func(t *testing.T) {
		_ = run(t, []string{
			"-id=" + role.ID,
			"-token=root",
			"-node-identity=foo:bar",
		})

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 2)
		require.Len(t, role.ServiceIdentities, 3)
		require.Len(t, role.NodeIdentities, 1)
	})
}

func TestRoleUpdateCommand_JSON(t *testing.T) {
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

	// Create policy
	policy1, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy1"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	role, _, err := client.ACL().RoleCreate(
		&api.ACLRole{
			Name: "test-role",
			ServiceIdentities: []*api.ACLServiceIdentity{
				{
					ServiceName: "fake",
				},
			},
		},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	t.Run("update a role that does not exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + fakeID,
			"-token=root",
			"-policy-name=" + policy1.Name,
			"-description=test role edited",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Role not found with ID")
	})

	t.Run("update with policy by name", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-token=root",
			"-policy-name=" + policy1.Name,
			"-description=test role edited",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		var jsonOutput json.RawMessage
		err := json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
		assert.NoError(t, err)
	})
}

func TestRoleUpdateCommand_noMerge(t *testing.T) {
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

	// Create 3 policies
	policy1, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy1"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)
	policy2, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy2"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)
	policy3, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy3"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// create a role
	createRole := func(t *testing.T) *api.ACLRole {
		roleUnq, err := uuid.GenerateUUID()
		require.NoError(t, err)

		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{
				Name:        "test-role-" + roleUnq,
				Description: "original description",
				ServiceIdentities: []*api.ACLServiceIdentity{
					{
						ServiceName: "fake",
					},
				},
				Policies: []*api.ACLRolePolicyLink{
					{
						ID: policy3.ID,
					},
				},
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
		return role
	}

	t.Run("update a role that does not exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + fakeID,
			"-token=root",
			"-policy-name=" + policy1.Name,
			"-no-merge",
			"-description=test role edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Role not found with ID")
	})

	t.Run("update with policy by name", func(t *testing.T) {
		role := createRole(t)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-name=" + role.Name,
			"-token=root",
			"-no-merge",
			"-policy-name=" + policy1.Name,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "", role.Description)
		require.Len(t, role.Policies, 1)
		require.Len(t, role.ServiceIdentities, 0)
	})

	t.Run("update with policy by id", func(t *testing.T) {
		role := createRole(t)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-name=" + role.Name,
			"-token=root",
			"-no-merge",
			"-policy-id=" + policy2.ID,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "", role.Description)
		require.Len(t, role.Policies, 1)
		require.Len(t, role.ServiceIdentities, 0)
	})

	t.Run("update with service identity", func(t *testing.T) {
		role := createRole(t)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-name=" + role.Name,
			"-token=root",
			"-no-merge",
			"-service-identity=web",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "", role.Description)
		require.Len(t, role.Policies, 0)
		require.Len(t, role.ServiceIdentities, 1)
	})

	t.Run("update with service identity scoped to 2 DCs", func(t *testing.T) {
		role := createRole(t)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-name=" + role.Name,
			"-token=root",
			"-no-merge",
			"-service-identity=db:abc,xyz",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		role, _, err := client.ACL().RoleRead(
			role.ID,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, role)
		require.Equal(t, "", role.Description)
		require.Len(t, role.Policies, 0)
		require.Len(t, role.ServiceIdentities, 1)
	})
}
