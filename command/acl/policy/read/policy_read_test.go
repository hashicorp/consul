// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policyread

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
)

func TestPolicyReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestPolicyReadCommand(t *testing.T) {
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
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))

	ui := cli.NewMockUi()
	cmd := New(ui)

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(t, err)

	// Test querying by id field
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-id=" + policy.ID,
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	assert.Contains(t, output, fmt.Sprintf("test-policy"))
	assert.Contains(t, output, policy.ID)

	// Test querying by name field
	argsName := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-name=test-policy",
	}

	cmd = New(ui)
	code = cmd.Run(argsName)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	output = ui.OutputWriter.String()
	assert.Contains(t, output, fmt.Sprintf("test-policy"))
	assert.Contains(t, output, policy.ID)

	// Test querying non-existent policy
	argsName = []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-name=test-policy-not-exist",
	}

	cmd = New(ui)
	code = cmd.Run(argsName)
	assert.Equal(t, code, 1)
}

func TestPolicyReadCommand_JSON(t *testing.T) {
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
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WithToken("root"))

	ui := cli.NewMockUi()
	cmd := New(ui)

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-id=" + policy.ID,
		"-format=json",
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	assert.Contains(t, output, fmt.Sprintf("test-policy"))
	assert.Contains(t, output, policy.ID)

	var jsonOutput json.RawMessage
	err = json.Unmarshal([]byte(output), &jsonOutput)
	assert.NoError(t, err)
}
