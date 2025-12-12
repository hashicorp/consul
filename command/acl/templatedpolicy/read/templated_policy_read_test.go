// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package templatedpolicyread

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

func TestTemplatedPolicyReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTemplatedPolicyReadCommand(t *testing.T) {
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

	t.Run("missing name flag", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 1)
		assert.Contains(t, ui.ErrorWriter.String(), "Must specify the -name parameter")
	})

	t.Run("correct input", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + api.ACLTemplatedPolicyNodeName,
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, "Name: String - Required - The node name.")
		require.Contains(t, output, "consul acl token create -templated-policy builtin/node -var name:node-1")
	})
}

func TestTemplatedPolicyReadCommand_JSON(t *testing.T) {
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

	t.Run("missing name flag", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-format=json",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 1)
		assert.Contains(t, ui.ErrorWriter.String(), "Must specify the -name parameter")
	})

	t.Run("correct input", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + api.ACLTemplatedPolicyNodeName,
			"-format=json",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		var templatedPolicy api.ACLTemplatedPolicyResponse
		err := json.Unmarshal([]byte(output), &templatedPolicy)

		assert.NoError(t, err)
		assert.Equal(t, structs.ACLTemplatedPolicyNodeSchema, templatedPolicy.Schema)
		assert.Equal(t, api.ACLTemplatedPolicyNodeName, templatedPolicy.TemplateName)
	})
}
