package bindingruledelete

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	// activate testing auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/testauth"
)

func TestBindingRuleDeleteCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestBindingRuleDeleteCommand(t *testing.T) {
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

	createRule := func(t *testing.T) string {
		rule, _, err := client.ACL().BindingRuleCreate(
			&api.ACLBindingRule{
				AuthMethod:  "test",
				Description: "test rule",
				BindType:    api.BindingRuleBindTypeService,
				BindName:    "test-${serviceaccount.name}",
				Selector:    "serviceaccount.namespace==default",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)
		return rule.ID
	}

	createDupe := func(t *testing.T) string {
		for {
			// Check for 1-char duplicates.
			rules, _, err := client.ACL().BindingRuleList(
				"test",
				&api.QueryOptions{Token: "root"},
			)
			require.NoError(t, err)

			m := make(map[byte]struct{})
			for _, rule := range rules {
				c := rule.ID[0]

				if _, ok := m[c]; ok {
					return string(c)
				}
				m[c] = struct{}{}
			}

			_ = createRule(t)
		}
	}

	t.Run("id required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Must specify the -id parameter")
	})

	t.Run("delete works", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("deleted successfully"))
		require.Contains(t, output, id)

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.Nil(t, rule)
	})

	t.Run("delete works via prefixes", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id[0:5],
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, fmt.Sprintf("deleted successfully"))
		require.Contains(t, output, id)

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.Nil(t, rule)
	})

	t.Run("delete fails when prefix matches more than one rule", func(t *testing.T) {
		prefix := createDupe(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id=" + prefix,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Error determining binding rule ID")
	})
}
