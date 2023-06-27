// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bootstrap

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestBootstrapCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestBootstrapCommand_Pretty(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())
	output := ui.OutputWriter.String()
	assert.Contains(t, output, "Bootstrap Token")
	assert.Contains(t, output, structs.ACLPolicyGlobalManagementID)
}

func TestBootstrapCommand_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-format=json",
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())
	output := ui.OutputWriter.String()
	assert.Contains(t, output, "Bootstrap Token")
	assert.Contains(t, output, structs.ACLPolicyGlobalManagementID)

	var jsonOutput json.RawMessage
	err := json.Unmarshal([]byte(output), &jsonOutput)
	require.NoError(t, err, "token unmarshalling error")
}

func TestBootstrapCommand_Initial(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)

	// Create temp file
	f, err := os.CreateTemp("", "consul-token.token")
	assert.Nil(t, err)
	defer os.Remove(f.Name())

	// Write the token to the file
	err = os.WriteFile(f.Name(), []byte("2b778dd9-f5f1-6f29-b4b4-9a5fa948757a"), 0700)
	assert.Nil(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-format=json",
		f.Name(),
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())
	output := ui.OutputWriter.String()
	assert.Contains(t, output, "Bootstrap Token")
	assert.Contains(t, output, structs.ACLPolicyGlobalManagementID)
	assert.Contains(t, output, "2b778dd9-f5f1-6f29-b4b4-9a5fa948757a")

	var jsonOutput json.RawMessage
	err = json.Unmarshal([]byte(output), &jsonOutput)
	require.NoError(t, err, "token unmarshalling error")
}
