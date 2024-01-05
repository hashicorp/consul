// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bindingruleupdate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// activate testing auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/testauth"
)

func TestBindingRuleUpdateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestBindingRuleUpdateCommand(t *testing.T) {
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

	deleteRules := func(t *testing.T) {
		rules, _, err := client.ACL().BindingRuleList(
			"test",
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)

		for _, rule := range rules {
			_, err := client.ACL().BindingRuleDelete(
				rule.ID,
				&api.WriteOptions{Token: "root"},
			)
			require.NoError(t, err)
		}
	}

	t.Run("rule id required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Cannot update a binding rule without specifying the -id parameter")
	})

	t.Run("rule id partial matches nothing", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + fakeID[0:5],
			"-token=root",
			"-description=test rule edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Error determining binding rule ID")
	})

	t.Run("rule id exact match doesn't exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + fakeID,
			"-token=root",
			"-description=test rule edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Binding rule not found with ID")
	})

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

	t.Run("rule id partial matches multiple", func(t *testing.T) {
		prefix := createDupe(t)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + prefix,
			"-token=root",
			"-description=test rule edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Error determining binding rule ID")
	})

	t.Run("must use roughly valid selector", func(t *testing.T) {
		id := createRule(t)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-selector", "foo",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Selector is invalid")
	})

	t.Run("update all fields", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "role",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, api.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("update all fields - partial", func(t *testing.T) {
		deleteRules(t) // reset since we created a bunch that might be dupes

		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id[0:5],
			"-description=test rule edited",
			"-bind-type", "role",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, api.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("update all fields but description", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-bind-type", "role",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("update all fields but bind name", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "role",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "test-${serviceaccount.name}", rule.BindName)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("update all fields but must exist", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-description=test rule edited",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeService, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("update all fields but selector", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "role",
			"-bind-name=role-updated",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, "serviceaccount.namespace==default", rule.Selector)
	})

	t.Run("update all fields clear selector", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "role",
			"-bind-name=role-updated",
			"-selector=",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Empty(t, rule.Selector)
	})

	t.Run("update all fields json formatted", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "role",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, api.BindingRuleBindTypeRole, rule.BindType)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)

		output := ui.OutputWriter.String()
		var jsonOutput json.RawMessage
		err = json.Unmarshal([]byte(output), &jsonOutput)
		assert.NoError(t, err)
	})
}

func TestBindingRuleUpdateCommand_noMerge(t *testing.T) {
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

	deleteRules := func(t *testing.T) {
		rules, _, err := client.ACL().BindingRuleList(
			"test",
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)

		for _, rule := range rules {
			_, err := client.ACL().BindingRuleDelete(
				rule.ID,
				&api.WriteOptions{Token: "root"},
			)
			require.NoError(t, err)
		}
	}

	t.Run("rule id required", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
		}

		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Cannot update a binding rule without specifying the -id parameter")
	})

	t.Run("rule id partial matches nothing", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + fakeID[0:5],
			"-token=root",
			"-no-merge",
			"-description=test rule edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Error determining binding rule ID")
	})

	t.Run("rule id exact match doesn't exist", func(t *testing.T) {
		fakeID, err := uuid.GenerateUUID()
		require.NoError(t, err)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + fakeID,
			"-token=root",
			"-no-merge",
			"-description=test rule edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Binding rule not found with ID")
	})

	createRule := func(t *testing.T) string {
		rule, _, err := client.ACL().BindingRuleCreate(
			&api.ACLBindingRule{
				AuthMethod:  "test",
				Description: "test rule",
				BindType:    api.BindingRuleBindTypeRole,
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

	t.Run("rule id partial matches multiple", func(t *testing.T) {
		prefix := createDupe(t)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + prefix,
			"-token=root",
			"-no-merge",
			"-description=test rule edited",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Error determining binding rule ID")
	})

	t.Run("must use roughly valid selector", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "service",
			"-bind-name=role-updated",
			"-selector", "foo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Selector is invalid")
	})

	t.Run("update all fields", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "service",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeService, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("update all fields - partial", func(t *testing.T) {
		deleteRules(t) // reset since we created a bunch that might be dupes

		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-id", id[0:5],
			"-description=test rule edited",
			"-bind-type", "service",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeService, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("update all fields but description", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-id", id,
			"-bind-type", "service",
			"-bind-name=role-updated",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Empty(t, rule.Description)
		require.Equal(t, api.BindingRuleBindTypeService, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Equal(t, "serviceaccount.namespace==alt and serviceaccount.name==demo", rule.Selector)
	})

	t.Run("missing bind name", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-id=" + id,
			"-description=test rule edited",
			"-bind-type", "role",
			"-selector=serviceaccount.namespace==alt and serviceaccount.name==demo",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Missing required '-bind-name' flag")
	})

	t.Run("update all fields but selector", func(t *testing.T) {
		id := createRule(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-no-merge",
			"-id", id,
			"-description=test rule edited",
			"-bind-type", "service",
			"-bind-name=role-updated",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0, "err: %s", ui.ErrorWriter.String())
		require.Empty(t, ui.ErrorWriter.String())

		rule, _, err := client.ACL().BindingRuleRead(
			id,
			&api.QueryOptions{Token: "root"},
		)
		require.NoError(t, err)
		require.NotNil(t, rule)

		require.Equal(t, "test rule edited", rule.Description)
		require.Equal(t, api.BindingRuleBindTypeService, rule.BindType)
		require.Equal(t, "role-updated", rule.BindName)
		require.Empty(t, rule.Selector)
	})
}
