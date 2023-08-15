// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tokencreate

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestTokenCreateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenCreateCommand_Pretty(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	node_name = "test-node"
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

	// create with policy by name
	t.Run("policy-name", func(t *testing.T) {
		_ = run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
		})
	})

	// create with policy by id
	t.Run("policy-id", func(t *testing.T) {
		_ = run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-id=" + policy.ID,
			"-description=test token",
		})
	})

	// create with a node identity
	t.Run("node-identity", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-node-identity=" + a.Config.NodeName + ":" + a.Config.Datacenter,
		})

		conf := api.DefaultConfig()
		conf.Address = a.HTTPAddr()
		conf.Token = token.SecretID
		client, err := api.NewClient(conf)
		require.NoError(t, err)

		nodes, _, err := client.Catalog().Nodes(nil)
		require.NoError(t, err)
		require.Len(t, nodes, 1)
		require.Equal(t, a.Config.NodeName, nodes[0].Node)
	})

	// create with accessor and secret
	t.Run("predefined-ids", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-id=" + policy.ID,
			"-description=test token",
			"-accessor=3d852bb8-5153-4388-a3ca-8ca78661889f",
			"-secret=3a69a8d8-c4d4-485d-9b19-b5b61648ea0c",
		})

		require.Equal(t, "3d852bb8-5153-4388-a3ca-8ca78661889f", token.AccessorID)
		require.Equal(t, "3a69a8d8-c4d4-485d-9b19-b5b61648ea0c", token.SecretID)
	})

	// create with an expires-ttl (<24h)
	t.Run("expires-ttl_short", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
			"-expires-ttl=1h",
		})

		// check diff between creation and expires time since we
		// always set the token.ExpirationTTL value to 0 at the moment
		require.Equal(t, time.Hour, token.ExpirationTime.Sub(token.CreateTime))
	})

	// create with an expires-ttl long (>24h)
	t.Run("expires-ttl_long", func(t *testing.T) {
		token := run(t, []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-policy-name=" + policy.Name,
			"-description=test token",
			"-expires-ttl=8760h",
		})

		require.Equal(t, 8760*time.Hour, token.ExpirationTime.Sub(token.CreateTime))
	})
}

func TestTokenCreateCommand_JSON(t *testing.T) {
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
			"-policy-name=" + policy.Name,
			"-description=test token",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		var jsonOutput json.RawMessage
		err = json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
		require.NoError(t, err, "token unmarshalling error")
	}
}
