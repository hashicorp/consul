package authmethoddelete

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	// activate testing auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/testauth"
)

func TestAuthMethodDeleteCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestAuthMethodDeleteCommand(t *testing.T) {
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

	t.Run("name required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Must specify the -name parameter")
	})

	t.Run("delete notfound", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=notfound",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("deleted successfully"))
		require.Contains(t, output, "notfound")
	})

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "test-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "testing",
				Description: "test",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	t.Run("delete works", func(t *testing.T) {
		name := createAuthMethod(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("deleted successfully"))
		require.Contains(t, output, name)

		method, _, err := client.ACL().AuthMethodRead(
			name,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.Nil(t, method)
	})
}
