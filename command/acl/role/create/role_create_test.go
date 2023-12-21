package rolecreate

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
)

func TestRoleCreateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleCreateCommand_Pretty(t *testing.T) {
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

	run := func(t *testing.T, args []string) *api.ACLRole {
		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(append(args, "-format=json", "-http-addr="+a.HTTPAddr()))
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())

		var role api.ACLRole
		require.NoError(t, json.Unmarshal(ui.OutputWriter.Bytes(), &role))
		return &role
	}

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// create with policy by name
	t.Run("policy-name", func(t *testing.T) {
		_ = run(t, []string{
			"-token=root",
			"-name=role-with-policy-by-name",
			"-description=test-role",
			"-policy-name=" + policy.Name,
		})
	})

	// create with policy by id
	t.Run("policy-id", func(t *testing.T) {
		_ = run(t, []string{
			"-token=root",
			"-name=role-with-policy-by-id",
			"-description=test-role",
			"-policy-id=" + policy.ID,
		})
	})

	// create with service identity
	t.Run("service-identity", func(t *testing.T) {
		_ = run(t, []string{
			"-token=root",
			"-name=role-with-service-identity",
			"-description=test-role",
			"-service-identity=web",
		})
	})

	// create with service identity scoped to 2 DCs
	t.Run("dc-scoped-service-identity", func(t *testing.T) {
		_ = run(t, []string{
			"-token=root",
			"-name=role-with-service-identity-in-2-dcs",
			"-description=test-role",
			"-service-identity=db:abc,xyz",
		})
	})

	t.Run("node-identity", func(t *testing.T) {
		role := run(t, []string{
			"-token=root",
			"-name=role-with-node-identity",
			"-description=test-role",
			"-node-identity=foo:bar",
		})

		require.Len(t, role.NodeIdentities, 1)
	})
}

func TestRoleCreateCommand_JSON(t *testing.T) {
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
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		var jsonOutput json.RawMessage
		err = json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
		assert.NoError(t, err)
	}
}
