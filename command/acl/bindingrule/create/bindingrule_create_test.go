package bindingrulecreate

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

	// activate testing auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/testauth"
)

func TestBindingRuleCreateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestBindingRuleCreateCommand(t *testing.T) {
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

	// create an auth method in advance
	{
		_, _, err := client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name: "test",
				Type: "testing",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
	}

	t.Run("method is required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-method' flag")
	})

	t.Run("bind type required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-bind-type=",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-bind-type' flag")
	})

	t.Run("bind name required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-bind-type=service",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-bind-name' flag")
	})

	t.Run("must use roughly valid selector", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-bind-type=service",
			"-bind-name=demo",
			"-selector", "foo",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Selector is invalid")
	})

	t.Run("create it with no selector", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-bind-type=service",
			"-bind-name=demo",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
	})

	t.Run("create it with a match selector", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-bind-type=service",
			"-bind-name=demo",
			"-selector", "serviceaccount.namespace==default and serviceaccount.name==vault",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
	})

	t.Run("create it with type role", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-method=test",
			"-bind-type=role",
			"-bind-name=demo",
			"-selector", "serviceaccount.namespace==default and serviceaccount.name==vault",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
	})
}
