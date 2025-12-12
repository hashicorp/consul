// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package templatedpolicylist

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplatedPolicyListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTemplatedPolicyListCommand(t *testing.T) {
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
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	}

	cmd := New(ui)
	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	require.Contains(t, output, api.ACLTemplatedPolicyServiceName)
	require.Contains(t, output, api.ACLTemplatedPolicyDNSName)
}

func TestTemplatedPolicyListCommand_JSON(t *testing.T) {
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
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-format=json",
	}

	cmd := New(ui)
	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())
	output := ui.OutputWriter.String()
	require.Contains(t, output, api.ACLTemplatedPolicyServiceName)
	require.Contains(t, output, api.ACLTemplatedPolicyDNSName)

	var jsonOutput map[string]api.ACLTemplatedPolicyResponse
	err := json.Unmarshal([]byte(output), &jsonOutput)
	assert.NoError(t, err)
	outputTemplate := jsonOutput[api.ACLTemplatedPolicyDNSName]
	assert.Equal(t, structs.ACLTemplatedPolicyNoRequiredVariablesSchema, outputTemplate.Schema)
}
