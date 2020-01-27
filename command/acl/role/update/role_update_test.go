package roleupdate

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

	uuid "github.com/hashicorp/go-uuid"
)

func TestRoleUpdateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleUpdateCommand(t *testing.T) {
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
				&api.ACLServiceIdentity{
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
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 1)
		require.Len(t, role.ServiceIdentities, 1)
	})

	t.Run("update with policy by id", func(t *testing.T) {
		// also update with no description shouldn't delete the current
		// description
		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-token=root",
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
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 2)
		require.Len(t, role.ServiceIdentities, 1)
	})

	t.Run("update with service identity", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-token=root",
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
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 2)
		require.Len(t, role.ServiceIdentities, 2)
	})

	t.Run("update with service identity scoped to 2 DCs", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + role.ID,
			"-token=root",
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
		require.Equal(t, "test role edited", role.Description)
		require.Len(t, role.Policies, 2)
		require.Len(t, role.ServiceIdentities, 3)
	})
}

func TestRoleUpdateCommand_noMerge(t *testing.T) {
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
					&api.ACLServiceIdentity{
						ServiceName: "fake",
					},
				},
				Policies: []*api.ACLRolePolicyLink{
					&api.ACLRolePolicyLink{
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
