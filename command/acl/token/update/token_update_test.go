// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tokenupdate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestTokenUpdateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func create_token(t *testing.T, client *api.Client, aclToken *api.ACLToken, writeOptions *api.WriteOptions) *api.ACLToken {
	token, _, err := client.ACL().TokenCreate(aclToken, writeOptions)
	require.NoError(t, err)

	return token
}

func TestTokenUpdateCommand(t *testing.T) {
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

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	run := func(t *testing.T, args []string) *api.ACLToken {
		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(append(args, "-format=json"))
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())

		var token api.ACLToken
		require.NoError(t, json.Unmarshal(ui.OutputWriter.Bytes(), &token))
		return &token
	}

	// update with node identity
	t.Run("node-identity", func(t *testing.T) {
		token := create_token(t, client, &api.ACLToken{Description: "test"}, &api.WriteOptions{Token: "root"})

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-node-identity=foo:bar",
			"-description=test token",
		})

		require.Len(t, responseToken.NodeIdentities, 1)
		require.Equal(t, "foo", responseToken.NodeIdentities[0].NodeName)
		require.Equal(t, "bar", responseToken.NodeIdentities[0].Datacenter)
	})

	t.Run("node-identity-merge", func(t *testing.T) {
		token := create_token(t,
			client,
			&api.ACLToken{Description: "test", NodeIdentities: []*api.ACLNodeIdentity{{NodeName: "foo", Datacenter: "bar"}}},
			&api.WriteOptions{Token: "root"},
		)

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-node-identity=bar:baz",
			"-description=test token",
			"-merge-node-identities",
		})

		require.Len(t, responseToken.NodeIdentities, 2)
		expected := []*api.ACLNodeIdentity{
			{
				NodeName:   "foo",
				Datacenter: "bar",
			},
			{
				NodeName:   "bar",
				Datacenter: "baz",
			},
		}
		require.ElementsMatch(t, expected, responseToken.NodeIdentities)
	})

	// update with policy by name
	t.Run("policy-name", func(t *testing.T) {
		token := create_token(t, client, &api.ACLToken{Description: "test"}, &api.WriteOptions{Token: "root"})

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
		})

		require.Len(t, responseToken.Policies, 1)
	})

	// update with policy by id
	t.Run("policy-id", func(t *testing.T) {
		token := create_token(t, client, &api.ACLToken{Description: "test"}, &api.WriteOptions{Token: "root"})

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-policy-id=" + policy.ID,
			"-description=test token",
		})

		require.Len(t, responseToken.Policies, 1)
	})

	// update with service-identity
	t.Run("service-identity", func(t *testing.T) {
		token := create_token(t, client, &api.ACLToken{Description: "test"}, &api.WriteOptions{Token: "root"})

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-service-identity=service:datapalace",
			"-description=test token",
		})

		require.Len(t, responseToken.ServiceIdentities, 1)
		require.Equal(t, "service", responseToken.ServiceIdentities[0].ServiceName)
	})

	// update with no description shouldn't delete the current description
	t.Run("merge-description", func(t *testing.T) {
		token := create_token(t, client, &api.ACLToken{Description: "test token"}, &api.WriteOptions{Token: "root"})

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
		})

		require.Equal(t, "test token", responseToken.Description)
	})
}

func TestTokenUpdateCommandWithAppend(t *testing.T) {
	t.Skip("TODO: flaky")
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

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	//secondary policy
	secondPolicy, _, policyErr := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "secondary-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, policyErr)

	run := func(t *testing.T, args []string) *api.ACLToken {
		ui := cli.NewMockUi()
		cmd := New(ui)

		code := cmd.Run(append(args, "-format=json"))
		require.Equal(t, 0, code)
		require.Empty(t, ui.ErrorWriter.String())

		var token api.ACLToken
		require.NoError(t, json.Unmarshal(ui.OutputWriter.Bytes(), &token))
		return &token
	}

	// update with append-policy-name
	t.Run("append-policy-name", func(t *testing.T) {
		token := create_token(t, client,
			&api.ACLToken{Description: "test", Policies: []*api.ACLTokenPolicyLink{{Name: policy.Name}}},
			&api.WriteOptions{Token: "root"},
		)

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-append-policy-name=" + secondPolicy.Name,
			"-description=test token",
		})

		require.Len(t, responseToken.Policies, 2)
	})

	// update with append-policy-id
	t.Run("append-policy-id", func(t *testing.T) {
		token := create_token(t, client,
			&api.ACLToken{Description: "test", Policies: []*api.ACLTokenPolicyLink{{Name: policy.Name}}},
			&api.WriteOptions{Token: "root"},
		)

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-append-policy-id=" + secondPolicy.ID,
			"-description=test token",
		})

		require.Len(t, responseToken.Policies, 2)
	})

	// update with append-node-identity
	t.Run("append-node-identity", func(t *testing.T) {
		token := create_token(t, client,
			&api.ACLToken{
				Description:    "test",
				Policies:       []*api.ACLTokenPolicyLink{{Name: policy.Name}},
				NodeIdentities: []*api.ACLNodeIdentity{{NodeName: "namenode", Datacenter: "somewhere"}},
			},
			&api.WriteOptions{Token: "root"},
		)

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-append-node-identity=third:node",
			"-description=test token",
		})

		require.Len(t, responseToken.NodeIdentities, 2)
		require.Equal(t, "third", responseToken.NodeIdentities[1].NodeName)
		require.Equal(t, "node", responseToken.NodeIdentities[1].Datacenter)
	})

	// update with append-service-identity
	t.Run("append-service-identity", func(t *testing.T) {
		token := create_token(t, client,
			&api.ACLToken{
				Description:       "test",
				Policies:          []*api.ACLTokenPolicyLink{{Name: policy.Name}},
				ServiceIdentities: []*api.ACLServiceIdentity{{ServiceName: "service"}},
			},
			&api.WriteOptions{Token: "root"},
		)

		responseToken := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-append-service-identity=web",
			"-description=test token",
		})

		require.Len(t, responseToken.ServiceIdentities, 2)
		require.Equal(t, "web", responseToken.ServiceIdentities[1].ServiceName)
	})
}

func TestTokenUpdateCommand_JSON(t *testing.T) {
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

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	token := create_token(t, client, &api.ACLToken{Description: "test"}, &api.WriteOptions{Token: "root"})

	t.Run("update with policy by name", func(t *testing.T) {
		cmd := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
			"-format=json",
		}

		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())

		var jsonOutput json.RawMessage
		err := json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
		require.NoError(t, err, "token unmarshalling error")
	})
}
