// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agenttokens

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestAgentTokensCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestAgentTokensCommand(t *testing.T) {
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

	// Create a token to set
	client := a.Client()

	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(t, err)

	// default token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"default",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())
	}

	// agent token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"agent",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())
	}

	// recovery token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"recovery",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())
	}

	// replication token
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"replication",
			token.SecretID,
		}

		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())
	}
}
