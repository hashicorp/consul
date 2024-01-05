// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tokenread

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestTokenReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenReadCommand_Pretty(t *testing.T) {
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

	// Create a token
	client := a.Client()

	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-accessor-id=" + token.AccessorID,
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	assert.Contains(t, output, fmt.Sprintf("test"))
	assert.Contains(t, output, token.AccessorID)
	assert.Contains(t, output, token.SecretID)
}

func TestTokenReadCommand_JSON(t *testing.T) {
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

	// Create a token
	client := a.Client()

	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-accessor-id=" + token.AccessorID,
		"-format=json",
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	var jsonOutput json.RawMessage
	err = json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
	require.NoError(t, err, "token unmarshalling error")
}

func TestTokenReadCommand_Self(t *testing.T) {
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

	// Create a token
	client := a.Client()

	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=" + token.SecretID,
		"-self",
	}

	code := cmd.Run(args)
	assert.Equal(t, code, 0)
	assert.Empty(t, ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	assert.Contains(t, output, fmt.Sprintf("test"))
	assert.Contains(t, output, token.AccessorID)
	assert.Contains(t, output, token.SecretID)
}
