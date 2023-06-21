// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tokenlist

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenListCommand_Pretty(t *testing.T) {
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

	var tokenIds []string

	// Create a couple tokens to list
	client := a.Client()
	for i := 0; i < 5; i++ {
		description := fmt.Sprintf("test token %d", i)

		token, _, err := client.ACL().TokenCreate(
			&api.ACLToken{Description: description},
			&api.WriteOptions{Token: "root"},
		)
		tokenIds = append(tokenIds, token.AccessorID)

		assert.NoError(t, err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())
	output := ui.OutputWriter.String()

	for i, v := range tokenIds {
		assert.Contains(t, output, fmt.Sprintf("test token %d", i))
		assert.Contains(t, output, v)
	}
}

func TestTokenListCommand_JSON(t *testing.T) {
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

	var tokenIds []string

	// Create a couple tokens to list
	client := a.Client()
	for i := 0; i < 5; i++ {
		description := fmt.Sprintf("test token %d", i)

		token, _, err := client.ACL().TokenCreate(
			&api.ACLToken{Description: description},
			&api.WriteOptions{Token: "root"},
		)
		tokenIds = append(tokenIds, token.AccessorID)

		assert.NoError(t, err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-format=json",
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	var jsonOutput []api.ACLTokenListEntry
	err := json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
	require.NoError(t, err, "token unmarshalling error")

	respIDs := make([]string, 0, len(jsonOutput))
	for _, obj := range jsonOutput {
		respIDs = append(respIDs, obj.AccessorID)
	}
	require.Subset(t, respIDs, tokenIds)
}
