// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policylist

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestPolicyListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestPolicyListCommand(t *testing.T) {
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

	var policyIDs []string

	// Create a couple polices to list
	client := a.Client()
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("test-policy-%d", i)

		policy, _, err := client.ACL().PolicyCreate(
			&api.ACLPolicy{Name: name},
			&api.WriteOptions{Token: "root"},
		)
		policyIDs = append(policyIDs, policy.ID)

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

	for i, v := range policyIDs {
		assert.Contains(t, output, fmt.Sprintf("test-policy-%d", i))
		assert.Contains(t, output, v)
	}
}

func TestPolicyListCommand_JSON(t *testing.T) {
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

	var policyIDs []string

	// Create a couple polices to list
	client := a.Client()
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("test-policy-%d", i)

		policy, _, err := client.ACL().PolicyCreate(
			&api.ACLPolicy{Name: name},
			&api.WriteOptions{Token: "root"},
		)
		policyIDs = append(policyIDs, policy.ID)

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
	output := ui.OutputWriter.String()

	for i, v := range policyIDs {
		assert.Contains(t, output, fmt.Sprintf("test-policy-%d", i))
		assert.Contains(t, output, v)
	}

	var jsonOutput json.RawMessage
	err := json.Unmarshal([]byte(output), &jsonOutput)
	assert.NoError(t, err)
}
