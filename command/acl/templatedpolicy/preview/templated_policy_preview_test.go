// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package templatedpolicylist

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplatedPolicyPreviewCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTemplatedPolicyPreviewCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testDir := testutil.TempDir(t, "acl")

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

	t.Run("missing name and file flags", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 1)
		assert.Contains(t, ui.ErrorWriter.String(), "Cannot preview a templated policy without specifying -name or -file")
	})

	t.Run("missing required template variables", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=builtin/node",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 1)
		assert.Contains(t, ui.ErrorWriter.String(), "Failed to generate the templated policy preview")
	})

	t.Run("correct input", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=builtin/node",
			"-var=name:api",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())
		output := ui.OutputWriter.String()
		require.Contains(t, output, "synthetic policy generated from templated policy: builtin/node")
	})

	t.Run("correct input with file", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-file=" + testDir + "/templated-policy.hcl",
		}

		templatedPolicy := []byte("TemplatedPolicy \"builtin/service\" { Name = \"web\"}")
		err := os.WriteFile(testDir+"/templated-policy.hcl", templatedPolicy, 0644)
		require.NoError(t, err)

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())
		output := ui.OutputWriter.String()
		require.Contains(t, output, "synthetic policy generated from templated policy: builtin/service")
	})

	t.Run("multiple templated policies input in file", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-file=" + testDir + "/templated-policy.hcl",
		}

		templatedPolicy := []byte(`
			TemplatedPolicy "builtin/service" { Name = "web"}
			TemplatedPolicy "builtin/node" { Name = "api"}
		`)
		err := os.WriteFile(testDir+"/templated-policy.hcl", templatedPolicy, 0644)
		require.NoError(t, err)

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 1)
		assert.Contains(t, ui.ErrorWriter.String(), "Can only preview a single templated policy at a time.")
	})
}

func TestTemplatedPolicyPreviewCommand_JSON(t *testing.T) {
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

	t.Run("missing templated-policy flags", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-format=json",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 1)
		assert.Contains(t, ui.ErrorWriter.String(), "Cannot preview a templated policy without specifying -name or -file")
	})

	t.Run("missing required template variables", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=builtin/node",
			"-format=json",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 1)
		assert.Contains(t, ui.ErrorWriter.String(), "Failed to generate the templated policy preview")
	})

	t.Run("correct input", func(t *testing.T) {
		ui := cli.NewMockUi()
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=builtin/node",
			"-var=name:api",
			"-format=json",
		}

		cmd := New(ui)
		code := cmd.Run(args)
		assert.Equal(t, code, 0)
		assert.Empty(t, ui.ErrorWriter.String())
		output := ui.OutputWriter.String()
		require.Contains(t, output, "synthetic policy generated from templated policy: builtin/node")

		// ensure valid json
		var jsonOutput json.RawMessage
		err := json.Unmarshal([]byte(output), &jsonOutput)
		assert.NoError(t, err)
	})
}
