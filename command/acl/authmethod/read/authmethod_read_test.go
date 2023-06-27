// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package authmethodread

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/agent"
	// activate testing auth method
	_ "github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestAuthMethodReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestAuthMethodReadCommand(t *testing.T) {
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

	client := a.Client()

	t.Run("name required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Must specify the -name parameter")
	})

	t.Run("not found", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=notfound",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 1)
		require.Contains(t, ui.ErrorWriter.String(), "Auth method not found with name")
	})

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "test-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "testing",
				Description: "test",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	t.Run("read by name", func(t *testing.T) {
		name := createAuthMethod(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, name)
	})
}

func TestAuthMethodReadCommand_JSON(t *testing.T) {
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

	client := a.Client()

	createAuthMethod := func(t *testing.T) string {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)

		methodName := "test-" + id

		_, _, err = client.ACL().AuthMethodCreate(
			&api.ACLAuthMethod{
				Name:        methodName,
				Type:        "testing",
				Description: "test",
			},
			&api.WriteOptions{Token: "root"},
		)
		require.NoError(t, err)

		return methodName
	}

	t.Run("read by name", func(t *testing.T) {
		name := createAuthMethod(t)

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-name=" + name,
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		output := ui.OutputWriter.String()
		require.Contains(t, output, name)

		var jsonOutput json.RawMessage
		err := json.Unmarshal([]byte(output), &jsonOutput)
		assert.NoError(t, err)
	})
}
